//go:build enterprise
// +build enterprise

// Copyright (c) 2026 Dugble Limited. All rights reserved.
//
// This file is part of Shiyao Enterprise and is not covered by the MIT License
// that governs the rest of the Shiyao repository.
//
// This code is proprietary and confidential. Unauthorized copying, distribution,
// modification, or use of this file, in any medium, is strictly prohibited.
// Use of this code requires a valid commercial license agreement with the copyright holder.

package saml

import "github.com/coffeyvidzro/shiyao/internal/enterprise/identity"

// Provider implements the identity provider interface for SAML.
type Provider struct {
	config Config
}

func NewProvider(config Config) *Provider {
	return &Provider{config: config}
}

func (p *Provider) Name() string {
	return "saml"
}

var _ identity.Provider = (*Provider)(nil)
