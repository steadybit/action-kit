package utils

import (
	"github.com/google/uuid"
	"testing"
)

func Test_ShortenUUID(t *testing.T) {
	tests := []struct {
		name string
		uuid uuid.UUID
		want string
	}{
		{
			name: "valid UUID",
			uuid: uuid.MustParse("85bd3ad3-b174-423f-9254-fe381a46da14"),
			want: "fe381a46da14",
		},
		{
			name: "another valid UUID",
			uuid: uuid.MustParse("987e6543-e21b-12d3-a456-426614174000"),
			want: "426614174000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShortenUUID(tt.uuid); got != tt.want {
				t.Errorf("ShortenUUID() = %v, want %v", got, tt.want)
			}
		})
	}
}
