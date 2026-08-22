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

package identity

// Provider describes the contract for an external identity provider integration.
type Provider interface {
	Name() string
}
