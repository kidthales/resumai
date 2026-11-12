package mcp_test

import (
	"reflect"
	"testing"

	"github.com/kidthales/resumai/internal/vault"
)

// TestVaultInterfaceAirGap verifies that the vault.Vault interface consumed by
// the MCP Server package strictly enforces the PII air-gap. Contact PII methods
// (GetPII, SetPII) must never be present in the Vault interface exposed to MCP.
func TestVaultInterfaceAirGap(t *testing.T) {
	vaultType := reflect.TypeOf((*vault.Vault)(nil)).Elem()

	for i := 0; i < vaultType.NumMethod(); i++ {
		method := vaultType.Method(i)
		if method.Name == "GetPII" || method.Name == "SetPII" {
			t.Fatalf("air-gap violation: vault.Vault interface exposes PII method %q to MCP", method.Name)
		}
	}
}
