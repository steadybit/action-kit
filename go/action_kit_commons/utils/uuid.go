package utils

import "github.com/google/uuid"

func ShortenUUID(uuid uuid.UUID) string {
	return uuid.String()[24:]
}
