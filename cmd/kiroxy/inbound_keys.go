package main

import (
	"context"
	"time"

	"local/kiroxy/internal/server"
	"local/kiroxy/internal/tokenvault"
)

// inboundKeyProvider adapts tokenvault.Vault to server.InboundKeyProvider.
// When the vault is nil (kiro-cli SQLite mode), Pass nil to server.Options
// and the dashboard endpoints 404.
type inboundKeyProvider struct {
	vault *tokenvault.Vault
}

func newInboundKeyProvider(v *tokenvault.Vault) *inboundKeyProvider {
	if v == nil {
		return nil
	}
	return &inboundKeyProvider{vault: v}
}

func (p *inboundKeyProvider) List(ctx context.Context) ([]server.InboundKeyView, error) {
	keys, err := p.vault.ListInboundKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]server.InboundKeyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, toInboundKeyView(k))
	}
	return out, nil
}

func (p *inboundKeyProvider) Create(ctx context.Context, label string) (*server.CreatedInboundKey, error) {
	id, plain, err := p.vault.CreateInboundKey(ctx, label)
	if err != nil {
		return nil, err
	}
	// Re-read the row so we have an accurate tail + timestamps. The vault
	// doesn't return the persisted row from Create to keep that API lean.
	keys, err := p.vault.ListInboundKeys(ctx)
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k.ID == id {
			return &server.CreatedInboundKey{
				InboundKeyView: toInboundKeyView(k),
				Plaintext:      plain,
			}, nil
		}
	}
	// Fallback — the ID is guaranteed to be present since Create succeeded,
	// but if something raced we still return a useful response.
	tail := ""
	if len(plain) >= 4 {
		tail = plain[len(plain)-4:]
	}
	return &server.CreatedInboundKey{
		InboundKeyView: server.InboundKeyView{
			ID:        id,
			Label:     label,
			Tail:      tail,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Plaintext: plain,
	}, nil
}

func (p *inboundKeyProvider) Revoke(ctx context.Context, id string) error {
	return p.vault.RevokeInboundKey(ctx, id)
}

func toInboundKeyView(k tokenvault.InboundKey) server.InboundKeyView {
	v := server.InboundKeyView{
		ID:        k.ID,
		Label:     k.Label,
		Tail:      k.Tail,
		Revoked:   k.Revoked,
		CreatedAt: k.CreatedAt.UTC().Format(time.RFC3339),
	}
	if !k.LastUsedAt.IsZero() {
		v.LastUsedAt = k.LastUsedAt.UTC().Format(time.RFC3339)
	}
	return v
}
