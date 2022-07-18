package attack_kit_api

import (
	"testing"
)

// Note: These test cases only check that the code compiles as intended.
// On compilation errors, we most likely have caused a breaking change of
// the API and need to act accordingly.

func markAsUsed(t *testing.T, v any) {
	if v == nil {
		t.Fail()
	}
}

func TestPrepareAttackRequestBody(t *testing.T) {
	v := PrepareAttackRequestBody{
		Config: make(map[string]interface{}),
		Target: Target{
			Name:       "gateway",
			Attributes: make(map[string][]string),
		},
	}
	markAsUsed(t, v)
}

func TestStartAttackRequestBody(t *testing.T) {
	v := StartAttackRequestBody{
		State: make(map[string]interface{}),
	}
	markAsUsed(t, v)
}

func TestAttackStatusRequestBody(t *testing.T) {
	v := AttackStatusRequestBody{
		State: make(map[string]interface{}),
	}
	markAsUsed(t, v)
}

func TestStopAttackRequestBody(t *testing.T) {
	v := StopAttackRequestBody{
		State: make(map[string]interface{}),
	}
	markAsUsed(t, v)
}

func TestAttackList(t *testing.T) {
	v := AttackList{
		Attacks: []DescribingEndpointReference{
			{
				"GET",
				"/attacks/rollout-restart",
			},
		},
	}
	markAsUsed(t, v)
}

func TestAttackDescription(t *testing.T) {
	v := AttackDescription{
		Id:          "com.steadybit.example.attacks.kubernetes.rollout-restart",
		Label:       "Rollout Restart Deployment",
		Description: "Execute a rollout restart for a Kubernetes deployment",
		Icon:        Ptr("data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%2224%22%20height%3D%2224%22%20viewBox%3D%220%200%2024%2024%22%3E%3Cpath%20d%3D%22M13.95%2013.5h-.23c-.18.11-.26.32-.18.5l.86%202.11c.83-.53%201.46-1.32%201.79-2.25l-2.23-.36h-.01m-3.45.29a.415.415%200%2000-.38-.29h-.08l-2.22.37c.33.92.96%201.7%201.79%202.23l.85-2.07V14c.04-.05.04-.14.04-.21m1.83.81a.378.378%200%2000-.51-.15c-.07.05-.12.08-.15.15h-.01l-1.09%201.97c.78.26%201.62.31%202.43.12.14-.03.29-.07.43-.12l-1.09-1.97h-.01m3.45-4.57L14.1%2011.5l.01.03a.37.37%200%2000-.04.53c.05.06.11.1.18.12l.01.01%202.17.62c.07-.97-.14-1.95-.65-2.78m-3.11.16c.01.21.18.37.39.36.08%200%20.15-.02.21-.05h.01l1.83-1.31a4.45%204.45%200%2000-2.57-1.24l.13%202.24m-1.94.31c.17.11.4.08.52-.09.05-.06.07-.13.08-.21h.01l.12-2.25c-.15.02-.3.05-.46.08-.8.18-1.54.58-2.12%201.16l1.84%201.31h.01m-.99%201.69c.2-.05.32-.26.26-.46%200-.08-.05-.14-.11-.19v-.01L8.21%2010c-.52.86-.74%201.84-.63%202.82l2.16-.62v-.01m1.64.66l.62.3.62-.3.15-.67-.43-.53h-.69l-.43.53.16.67m10.89%201.32L20.5%206.5c-.09-.42-.37-.76-.74-.94l-7.17-3.43c-.37-.17-.81-.17-1.19%200L4.24%205.56c-.37.18-.65.52-.74.94l-1.77%207.67c-.05.2-.05.4%200%20.59.01.06.03.12.05.18.03.09.08.19.13.27.03.04.05.08.09.11l4.95%206.18c.02%200%20.05.04.05.06.1.09.19.16.28.22.12.08.26.14.4.17.11.05.23.05.32.05h8.12c.07%200%20.14-.03.2-.05.05-.01.1-.03.14-.04.04-.02.07-.03.11-.05.05-.02.1-.05.15-.08.12-.08.23-.18.33-.28l.15-.2%204.8-5.98c.1-.12.17-.25.22-.38.02-.06.04-.12.05-.18.05-.19.05-.4%200-.59m-7.43%202.99c.02.06.04.12.07.17-.04.08-.06.17-.03.26.12.24.23.46.38.68.08.11.16.23.24.34%200%20.03.03.08.04.12.12.2.06.46-.15.59s-.47.05-.59-.15c-.01-.03-.02-.05-.03-.08-.02-.03-.04-.09-.06-.09-.05-.15-.09-.28-.12-.41-.09-.25-.17-.49-.3-.72a.375.375%200%2000-.21-.14l-.08-.16c-1.29.48-2.7.48-3.97-.01l-.1.18c-.07.01-.14.04-.19.09-.14.24-.24.49-.33.77-.03.13-.07.26-.12.4-.02%200-.04.07-.06.1a.43.43%200%2001-.81-.29c.01-.03.03-.05.04-.08.04-.03.04-.08.04-.11.09-.12.16-.23.24-.35.16-.21.29-.45.39-.69a.54.54%200%2000-.03-.25l.07-.18a5.611%205.611%200%2001-2.47-3.09l-.2.03a.388.388%200%2000-.23-.09c-.27.05-.51.13-.77.22-.11.06-.24.11-.37.15-.03.01-.07.02-.13.03a.438.438%200%2001-.54-.27c-.07-.23.04-.47.28-.55.02%200%20.05-.01.08-.01v-.01h.01l.11-.02c.14-.04.28-.04.41-.04.26%200%20.52-.06.77-.12.08-.05.14-.11.19-.19l.19-.05c-.21-1.36.1-2.73.86-3.87l-.14-.12c0-.09-.03-.18-.08-.25-.2-.17-.41-.32-.64-.45-.12-.06-.24-.13-.36-.21-.02-.02-.06-.05-.08-.07l-.01-.01c-.2-.16-.25-.42-.11-.63.09-.1.21-.15.35-.15.11.01.21.05.3.12l.09.07c.1.09.19.2.28.3.18.19.37.37.58.52.08.04.17.05.26.03l.15.11c.75-.8%201.73-1.36%202.8-1.6.25-.06.52-.1.78-.12l.01-.18a.45.45%200%2000.14-.23c.01-.26-.01-.52-.05-.77-.03-.13-.05-.27-.06-.41V5.1c-.02-.24.15-.45.39-.48s.44.15.47.38v.22c-.01.14-.03.28-.06.41-.04.25-.06.51-.05.77.02.1.07.17.14.22l.01.19c1.36.12%202.62.73%203.56%201.72l.16-.12c.09.02.18.01.26-.03.21-.15.41-.33.58-.52.09-.1.18-.2.28-.3.03-.02.07-.06.1-.06.17-.18.44-.18.59%200%20.19.16.18.43%200%20.6%200%20.02-.03.04-.06.06a2.495%202.495%200%2001-.44.28c-.23.13-.45.28-.64.45-.06.07-.09.15-.08.24l-.16.14a5.44%205.44%200%2001.88%203.86l.19.05c.04.08.11.14.19.18.25.07.51.11.77.14h.41c.03.03.08.04.12.05.24.03.4.25.37.49-.05.23-.24.4-.48.37-.03-.01-.07-.01-.07-.02v-.01c-.06%200-.1-.01-.14-.02-.13-.04-.25-.09-.36-.15-.26-.1-.5-.17-.77-.21-.09%200-.17%200-.23.08-.07-.01-.13-.02-.19-.03-.41%201.31-1.31%202.41-2.47%203.11z%22%20fill%3D%22currentcolor%22%2F%3E%3C%2Fsvg%3E"),
		Version:     "1.1.0",
		Category:    "state",
		TargetType:  "kubernetes-deployment",
		TimeControl: "INTERNAL",
		Parameters: []AttackParameter{
			{
				Label:        "Wait for rollout completion",
				Name:         "wait",
				Type:         Boolean,
				Advanced:     Ptr(true),
				DefaultValue: Ptr("false"),
			},
		},
		Prepare: MutatingEndpointReference{
			"POST",
			"/attacks/rollout-restart/prepare",
		},
		Start: MutatingEndpointReference{
			"POST",
			"/attacks/rollout-restart/start",
		},
		Status: Ptr(MutatingEndpointReferenceWithCallInterval{
			Method: "POST",
			Path:   "/attacks/rollout-restart/status",
		}),
		Stop: Ptr(MutatingEndpointReference{
			"POST",
			"/attacks/rollout-restart/stop",
		}),
	}
	markAsUsed(t, v)
}

func TestAttackStateAndMessages(t *testing.T) {
	v := AttackStateAndMessages{
		State: make(map[string]interface{}),
		Messages: Ptr([]Message{
			{
				Level:   Ptr(Debug),
				Message: "test",
			},
			{
				Level:   Ptr(Error),
				Message: "test",
			},
			{
				Level:   Ptr(Info),
				Message: "test",
			},
		}),
	}
	markAsUsed(t, v)
}

func TestKeyedMessages(t *testing.T) {
	v := KeyedMessages{
		Messages: Ptr([]Message{
			{
				Level:   Ptr(Debug),
				Message: "test",
			},
			{
				Level:   Ptr(Error),
				Message: "test",
			},
			{
				Level:   Ptr(Info),
				Message: "test",
			},
		}),
	}
	markAsUsed(t, v)
}

func TestAttackStatus(t *testing.T) {
	v := AttackStatus{
		State:     Ptr(make(map[string]interface{})),
		Completed: true,
		Messages: Ptr([]Message{
			{
				Level:   Ptr(Debug),
				Message: "test",
			},
			{
				Level:   Ptr(Error),
				Message: "test",
			},
			{
				Level:   Ptr(Info),
				Message: "test",
			},
		}),
	}
	markAsUsed(t, v)
}

func TestAttackKitError(t *testing.T) {
	v := AttackKitError{
		Detail:   Ptr("d"),
		Instance: Ptr("i"),
		Title:    "t",
		Type:     Ptr("t"),
	}
	markAsUsed(t, v)
}
