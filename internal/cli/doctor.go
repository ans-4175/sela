package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ans-4175/sela/internal/credentials"
)

// doctor never refreshes credentials or invokes a Google API. Keyring access is opt-in.
func (a *App) doctor(ctx context.Context, args []string) error {
	alias := ""
	inspect := false
	if err := parse("doctor", args, map[string]*string{"account": &alias}, nil, map[string]*bool{"credential-check": &inspect}); err != nil {
		return err
	}
	if inspect && alias == "" {
		return errors.New("--credential-check requires explicit --account")
	}
	result := map[string]any{"platform": runtime.GOOS + "/" + runtime.GOARCH, "network_used": false, "live_identity_checked": false, "credential_checked": false, "refresh_attempted": false, "protected_storage_supported": runtime.GOOS == "darwin" || runtime.GOOS == "linux"}
	if alias == "" {
		result["status"] = "discovery_only"
		return encode(a.Out, result)
	}
	if runtime.GOOS == "windows" {
		return errors.New("Windows private storage is not enabled; doctor discovery only")
	}
	path, err := a.registry()
	if err != nil {
		return err
	}
	result["account_alias"] = alias
	// Avoid creating absent configuration directories through the normal store reader.
	if _, err = os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		result["status"] = "account_setup_required"
		return encode(a.Out, result)
	}
	account, err := a.account(alias)
	if err != nil {
		result["status"] = "account_or_storage_invalid"
		return encode(a.Out, result)
	}
	connPath := filepath.Join(filepath.Dir(path), "connections", account.Name+".json")
	if _, err = os.Lstat(connPath); err != nil {
		result["status"] = "login_setup_required"
		return encode(a.Out, result)
	}
	_, cfg, err := a.connection(account)
	if err != nil {
		result["status"] = "connection_or_storage_invalid"
		return encode(a.Out, result)
	}
	result["configured_scopes"] = cfg.Scopes
	result["status"] = "local_configuration_ok"
	if !inspect {
		result["credential_status"] = "not_checked"
		return encode(a.Out, result)
	}
	if err = ctx.Err(); err != nil {
		return errors.New("doctor cancelled")
	}
	result["credential_checked"] = true
	if a.Credentials == nil {
		result["credential_status"] = "unavailable"
		result["status"] = "credential_attention_required"
		return encode(a.Out, result)
	}
	token, err := a.Credentials.Load(account.CredentialKey())
	if err != nil || token == nil {
		status := "unavailable_or_invalid"
		if errors.Is(err, credentials.ErrNotFound) {
			status = "missing"
		}
		result["credential_status"] = status
		result["status"] = "credential_attention_required"
		return encode(a.Out, result)
	}
	result["refresh_credential_present"] = token.RefreshToken != ""
	result["refresh_credential_validity"] = "unknown_until_authorized_online_use"
	status := "valid_locally_not_provider_verified"
	if token.AccessToken == "" {
		status = "access_credential_missing"
	} else if token.Expiry.IsZero() {
		status = "expiry_unknown"
	} else if !token.Valid() {
		status = "expired_or_expiring"
	}
	if token.RefreshToken == "" {
		status = "refresh_credential_missing"
	}
	result["credential_status"] = status
	if !token.Expiry.IsZero() {
		result["access_expires_at"] = token.Expiry.UTC().Format(time.RFC3339)
	}
	return encode(a.Out, result)
}
