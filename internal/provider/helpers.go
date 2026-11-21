package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

// configureClient extracts the API client the provider handed over. It
// returns nil before the provider is configured (validation-time calls).
func configureClient(providerData any, diags *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}

	api, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T. This is a bug in the provider.", providerData))

		return nil
	}

	return api
}

// importByOrgUID implements `terraform import <addr> org/uid` (a bare `uid`
// is accepted too). The org must be the one the provider is configured with.
func importByOrgUID(
	ctx context.Context, api *client.Client, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	parts := strings.Split(req.ID, "/")

	if len(parts) > 2 || parts[len(parts)-1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `org/uid`, got `"+req.ID+"`.")

		return
	}

	if len(parts) == 2 && !checkImportOrg(api, parts[0], &resp.Diagnostics) {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[len(parts)-1])...)
}

func checkImportOrg(api *client.Client, org string, diags *diag.Diagnostics) bool {
	if api != nil && org != api.Org() {
		diags.AddError("Organization mismatch", fmt.Sprintf(
			"The import ID names the organization `%s`, but the provider is configured for `%s`.", org, api.Org()))

		return false
	}

	return true
}

// decodeJSONObject parses a JSON-object attribute. A null or unknown value
// gives a nil map.
func decodeJSONObject(value jsontypes.Normalized, attr string, diags *diag.Diagnostics) map[string]any {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	dec := json.NewDecoder(strings.NewReader(value.ValueString()))
	dec.UseNumber()

	var out map[string]any

	if err := dec.Decode(&out); err != nil {
		diags.AddAttributeError(path.Root(attr), "Invalid JSON object",
			"`"+attr+"` must be a JSON object (use `jsonencode({...})`): "+err.Error())

		return nil
	}

	if out == nil {
		out = map[string]any{}
	}

	return out
}

func encodeJSONObject(value map[string]any, diags *diag.Diagnostics) jsontypes.Normalized {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(value); err != nil {
		diags.AddError("Cannot encode JSON", err.Error())

		return jsontypes.NewNormalizedNull()
	}

	return jsontypes.NewNormalizedValue(strings.TrimSpace(buf.String()))
}

// mergeSecrets builds the object sent to the API from its public and
// sensitive halves. Sensitive keys that were in the prior state and are gone
// from the plan are sent as an explicit null, which clears the stored secret
// (an omitted secret key is otherwise preserved by the server).
func mergeSecrets(public, sensitive, priorSensitive map[string]any) map[string]any {
	out := make(map[string]any, len(public)+len(sensitive))

	for k := range priorSensitive {
		out[k] = nil
	}

	for k, v := range public {
		out[k] = v
	}

	for k, v := range sensitive {
		out[k] = v
	}

	return out
}

// refreshPublic computes the public half to store after a read.
//
// The server never returns secret values: it strips them and lists their
// names in privateKeys. It may also add keys of its own (a generated signing
// secret, defaults). To keep the plan free of noise, only the keys the
// configuration manages are compared:
//   - no prior value (import): take what the server returns, minus the
//     secret keys (a few are returned in clear, like a webhook signing secret);
//   - key returned by the server: take the server value (drift is reported);
//   - key missing but listed in privateKeys: keep the prior value (a secret
//     written in the public attribute);
//   - key missing otherwise: dropped (it was removed out of band).
func refreshPublic(prior, server map[string]any, privateKeys []string) map[string]any {
	if prior == nil {
		out := make(map[string]any, len(server))

		for k, v := range server {
			if !slices.Contains(privateKeys, k) {
				out[k] = v
			}
		}

		return out
	}

	out := make(map[string]any, len(prior))

	for k, priorValue := range prior {
		if serverValue, ok := server[k]; ok {
			out[k] = serverValue
		} else if slices.Contains(privateKeys, k) {
			out[k] = priorValue
		}
	}

	return out
}

// refreshJSONAttr applies refreshPublic to a JSON attribute.
func refreshJSONAttr(
	prior jsontypes.Normalized, attr string, server map[string]any, privateKeys []string, diags *diag.Diagnostics,
) jsontypes.Normalized {
	priorMap := decodeJSONObject(prior, attr, diags)

	if priorMap == nil && len(server) == 0 {
		return jsontypes.NewNormalizedNull()
	}

	return encodeJSONObject(refreshPublic(priorMap, server, privateKeys), diags)
}

// optionalString maps an optional API string to state. The API does not tell
// "unset" from "empty", so an empty answer keeps whichever of null and ""
// the configuration used.
func optionalString(server string, prior types.String) types.String {
	if server != "" {
		return types.StringValue(server)
	}

	if !prior.IsNull() && !prior.IsUnknown() && prior.ValueString() == "" {
		return prior
	}

	return types.StringNull()
}

func deref(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

// stringOrEmpty returns the value to send for an optional string that the
// API clears with an empty string.
func stringOrEmpty(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}

	return value.ValueString()
}

// putString adds key to the payload when the value is set.
func putString(body client.Payload, key string, value types.String) {
	if !value.IsNull() && !value.IsUnknown() {
		body[key] = value.ValueString()
	}
}

// putBool adds key to the payload when the value is set.
func putBool(body client.Payload, key string, value types.Bool) {
	if !value.IsNull() && !value.IsUnknown() {
		body[key] = value.ValueBool()
	}
}

// putInt adds key to the payload when the value is set.
func putInt(body client.Payload, key string, value types.Int64) {
	if !value.IsNull() && !value.IsUnknown() {
		body[key] = value.ValueInt64()
	}
}

func addAPIError(diags *diag.Diagnostics, action string, err error) {
	diags.AddError("SolidPing API error", fmt.Sprintf("Could not %s: %s", action, err.Error()))
}
