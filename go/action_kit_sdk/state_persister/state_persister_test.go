// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package state_persister

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

type exampleState struct {
	stringValue string
	intValue    int
}

func TestInmemoryStatePersister_basics(t *testing.T) {
	persister := NewInmemoryStatePersister()
	exe1 := uuid.New()
	exe2 := uuid.New()

	err := persister.PersistState(context.Background(), &PersistedState{exe1, "action-1", &exampleState{"test", 1}})
	require.NoError(t, err)
	err = persister.PersistState(context.Background(), &PersistedState{exe2, "action-1", &exampleState{"test", 2}})
	require.NoError(t, err)

	states, err := persister.GetStates(context.Background())
	require.NoError(t, err)
	require.Len(t, states, 2)

	err = persister.DeleteState(context.Background(), exe1)
	require.NoError(t, err)

	states, err = persister.GetStates(context.Background())
	require.NoError(t, err)
	require.Len(t, states, 1)
}

func TestInmemoryStatePersister_should_ignore_not_found(t *testing.T) {
	persister := NewInmemoryStatePersister()
	exe1 := uuid.New()
	err := persister.PersistState(context.Background(), &PersistedState{exe1, "action-1", &exampleState{"test", 1}})
	require.NoError(t, err)

	err = persister.DeleteState(context.Background(), uuid.New())
	require.NoError(t, err)

	states, err := persister.GetStates(context.Background())
	require.NoError(t, err)
	require.Len(t, states, 1)
}

func TestInmemoryStatePersister_should_update_existing_values(t *testing.T) {
	persister := NewInmemoryStatePersister()
	exe1 := uuid.New()
	err := persister.PersistState(context.Background(), &PersistedState{exe1, "action-1", &exampleState{"test", 1}})
	require.NoError(t, err)

	err = persister.PersistState(context.Background(), &PersistedState{exe1, "action-1", &exampleState{"updated", 200}})
	require.NoError(t, err)

	states, err := persister.GetStates(context.Background())
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.Equal(t, "updated", states[0].State.(*exampleState).stringValue)
}