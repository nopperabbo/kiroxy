package kiroclient

import (
	"testing"

	"github.com/nopperabbo/kiroxy/internal/kiroproto"
)

func TestChooseAmzTarget_SwitchesOnProfileARN(t *testing.T) {
	tests := []struct {
		name    string
		payload *kiroproto.Payload
		want    string
	}{
		{"nil payload", nil, amzTargetAmazonQ},
		{"empty profile arn", &kiroproto.Payload{}, amzTargetAmazonQ},
		{"with profile arn", &kiroproto.Payload{ProfileARN: "arn:aws:codewhisperer:us-east-1:123:profile/x"}, amzTargetCodeWhisperer},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := chooseAmzTarget(tc.payload)
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}
