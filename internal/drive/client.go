// SPDX-License-Identifier: MPL-2.0
package drive

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	api "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const Scope = api.DriveScope
const FolderMIME = "application/vnd.google-apps.folder"
const DriveFields = "id,name,restrictions"
const FileFields = "id,name,description,mimeType,parents,driveId,trashed"
const PermissionFields = "id,type,role,emailAddress,domain,allowFileDiscovery,deleted,permissionDetails"

type Client struct {
	Service *api.Service
	Retries int
	Wait    func(context.Context, time.Duration) error
	// Google does not support concurrent permission mutations on the same file.
	PermissionMu sync.Mutex
}

func NewClient(ctx context.Context, h *http.Client, opts ...option.ClientOption) (*Client, error) {
	opts = append(opts, option.WithHTTPClient(h), option.WithUserAgent("terraform-provider-gdrive"))
	s, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{Service: s, Retries: 4, Wait: wait}, nil
}
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// APIError excludes Google's free-form error body, which can contain user data.
type APIError struct {
	Code   int
	Reason string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Google Drive API returned HTTP %d (%s)", e.Code, e.Reason)
}
func IsNotFound(err error) bool { var e *APIError; return errors.As(err, &e) && e.Code == 404 }

func (c *Client) call(ctx context.Context, safe bool, fn func() error) error {
	for attempt := 0; ; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var e *googleapi.Error
		if !errors.As(err, &e) {
			return errors.New("Google Drive request failed at the transport or authentication layer; verify credentials and connectivity")
		}
		reason := "requestFailed"
		limited := false
		for _, item := range e.Errors {
			switch item.Reason {
			case "rateLimitExceeded", "userRateLimitExceeded":
				limited = true
				reason = item.Reason
			case "notFound", "forbidden", "insufficientFilePermissions", "domainPolicy", "teamDriveMembershipRequired":
				reason = item.Reason
			}
		}
		sanitized := &APIError{e.Code, reason}
		transient := e.Code == 429 || e.Code == 500 || e.Code == 502 || e.Code == 503 || e.Code == 504 || (e.Code == 403 && limited)
		if !safe || !transient || attempt >= c.Retries {
			return sanitized
		}
		delay := time.Duration(1<<attempt)*time.Second + time.Duration(rand.IntN(250))*time.Millisecond
		if sec, err := strconv.Atoi(e.Header.Get("Retry-After")); err == nil && sec > 0 {
			delay = max(delay, time.Duration(sec)*time.Second)
		} else if at, err := http.ParseTime(e.Header.Get("Retry-After")); err == nil {
			delay = max(delay, time.Until(at))
		}
		if err := c.Wait(ctx, min(delay, 60*time.Second)); err != nil {
			return err
		}
	}
}
func (c *Client) CreateDrive(ctx context.Context, id string, v *api.Drive) (out *api.Drive, err error) {
	err = c.call(ctx, true, func() error {
		var e error
		out, e = c.Service.Drives.Create(id, v).Fields(DriveFields).Context(ctx).Do()
		return e
	})
	return
}
func (c *Client) GetDrive(ctx context.Context, id string, admin bool) (out *api.Drive, err error) {
	err = c.call(ctx, true, func() error {
		var e error
		out, e = c.Service.Drives.Get(id).UseDomainAdminAccess(admin).Fields(DriveFields).Context(ctx).Do()
		return e
	})
	return
}
func (c *Client) UpdateDrive(ctx context.Context, id string, v *api.Drive, admin bool) (out *api.Drive, err error) {
	err = c.call(ctx, true, func() error {
		var e error
		out, e = c.Service.Drives.Update(id, v).UseDomainAdminAccess(admin).Fields(DriveFields).Context(ctx).Do()
		return e
	})
	return
}
func (c *Client) DeleteDrive(ctx context.Context, id string, admin bool) error {
	// Never enable recursive item deletion, even with administrator access.
	return c.call(ctx, true, func() error {
		return c.Service.Drives.Delete(id).UseDomainAdminAccess(admin).AllowItemDeletion(false).Context(ctx).Do()
	})
}
func (c *Client) GetFile(ctx context.Context, id string) (out *api.File, err error) {
	err = c.call(ctx, true, func() error {
		var e error
		out, e = c.Service.Files.Get(id).SupportsAllDrives(true).Fields(FileFields).Context(ctx).Do()
		return e
	})
	return
}
func (c *Client) CreateFolder(ctx context.Context, v *api.File) (out *api.File, err error) {
	// File creation in Shared Drives has no idempotency key. Never blindly replay.
	err = c.call(ctx, false, func() error {
		var e error
		out, e = c.Service.Files.Create(v).SupportsAllDrives(true).Fields(FileFields).Context(ctx).Do()
		return e
	})
	return
}
func (c *Client) UpdateFolder(ctx context.Context, id string, v *api.File, add, remove string) (out *api.File, err error) {
	err = c.call(ctx, add == "" && remove == "", func() error {
		q := c.Service.Files.Update(id, v).SupportsAllDrives(true).Fields(FileFields).Context(ctx)
		if add != "" {
			q.AddParents(add)
		}
		if remove != "" {
			q.RemoveParents(remove)
		}
		var e error
		out, e = q.Do()
		return e
	})
	return
}
func (c *Client) GetPermission(ctx context.Context, file, id string, admin bool) (out *api.Permission, err error) {
	err = c.call(ctx, true, func() error {
		var e error
		out, e = c.Service.Permissions.Get(file, id).SupportsAllDrives(true).UseDomainAdminAccess(admin).Fields(PermissionFields).Context(ctx).Do()
		return e
	})
	return
}
func (c *Client) CreatePermission(ctx context.Context, file string, v *api.Permission, admin, notify bool) (out *api.Permission, err error) {
	err = c.call(ctx, false, func() error {
		var e error
		call := c.Service.Permissions.Create(file, v).SupportsAllDrives(true).UseDomainAdminAccess(admin).Fields(PermissionFields).Context(ctx)
		if v.Type == "user" || v.Type == "group" {
			call.SendNotificationEmail(notify)
		}
		out, e = call.Do()
		return e
	})
	return
}
func (c *Client) UpdatePermission(ctx context.Context, file, id string, v *api.Permission, admin bool) (out *api.Permission, err error) {
	err = c.call(ctx, true, func() error {
		var e error
		out, e = c.Service.Permissions.Update(file, id, v).SupportsAllDrives(true).UseDomainAdminAccess(admin).Fields(PermissionFields).Context(ctx).Do()
		return e
	})
	return
}
func (c *Client) DeletePermission(ctx context.Context, file, id string, admin bool) error {
	return c.call(ctx, true, func() error {
		return c.Service.Permissions.Delete(file, id).SupportsAllDrives(true).UseDomainAdminAccess(admin).Context(ctx).Do()
	})
}
func (c *Client) Permissions(ctx context.Context, file string, admin bool) ([]*api.Permission, error) {
	var out []*api.Permission
	token := ""
	seen := map[string]bool{}
	for {
		var page *api.PermissionList
		err := c.call(ctx, true, func() error {
			var e error
			call := c.Service.Permissions.List(file).SupportsAllDrives(true).UseDomainAdminAccess(admin).PageSize(100).Fields(googleapi.Field("nextPageToken,permissions(" + PermissionFields + ")"))
			// The SDK sends PageToken("") as pageToken=, which Drive rejects as an expired token.
			if token != "" {
				call = call.PageToken(token)
			}
			page, e = call.Context(ctx).Do()
			return e
		})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Permissions...)
		if page.NextPageToken == "" {
			return out, nil
		}
		token = page.NextPageToken
		if seen[token] {
			return nil, errors.New("Google Drive returned a repeated page token")
		}
		seen[token] = true
	}
}
func DirectRole(p *api.Permission) (string, bool) {
	if len(p.PermissionDetails) == 0 {
		return p.Role, true
	}
	for _, d := range p.PermissionDetails {
		if !d.Inherited {
			return d.Role, true
		}
	}
	return "", false
}
