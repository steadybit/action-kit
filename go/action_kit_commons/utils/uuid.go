// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import "github.com/google/uuid"

func ShortenUUID(uuid uuid.UUID) string {
	return uuid.String()[24:]
}
