// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// This test creates and deletes a real, empty Shared Drive. It is never enabled
// by regular unit/protocol test commands or pull-request CI.
func TestAccSharedDrive(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("GDRIVE_ACC_CREATE_DRIVE") != "1" {
		t.Skip("requires TF_ACC=1 and GDRIVE_ACC_CREATE_DRIVE=1 with dedicated Workspace credentials")
	}
	name := fmt.Sprintf("gdrive-acceptance-%d", time.Now().UnixNano())
	config := func(suffix string) string {
		return fmt.Sprintf(`
provider "gdrive" { use_domain_admin_access = true }
resource "gdrive_shared_drive" "test" {
  name = %q
  deletion_protection = false
}
`, name+suffix)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"gdrive": providerserver.NewProtocol6WithError(New("acceptance")())},
		Steps: []resource.TestStep{
			{Config: config(""), Check: resource.TestCheckResourceAttr("gdrive_shared_drive.test", "name", name)},
			{ResourceName: "gdrive_shared_drive.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"deletion_protection"}},
			{Config: config("-renamed"), Check: resource.TestCheckResourceAttr("gdrive_shared_drive.test", "name", name+"-renamed")},
		},
	})
}
