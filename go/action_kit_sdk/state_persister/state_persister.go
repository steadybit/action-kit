/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package state_persister

import (
	"context"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
)

type PersistedState struct {
	ExecutionId uuid.UUID
	ActionId    string
	State       action_kit_api.ActionState
}

type StatePersister interface {
	PersistState(ctx context.Context, state *PersistedState) error
	GetStates(ctx context.Context) ([]*PersistedState, error)
	DeleteState(ctx context.Context, executionId uuid.UUID) error
}

func NewInmemoryStatePersister() StatePersister {
	return &inmemoryStatePersister{states: make(map[uuid.UUID]*PersistedState)}
}

type inmemoryStatePersister struct {
	states map[uuid.UUID]*PersistedState
}

func (p *inmemoryStatePersister) PersistState(_ context.Context, state *PersistedState) error {
	p.states[state.ExecutionId] = state
	return nil
}
func (p *inmemoryStatePersister) GetStates(_ context.Context) ([]*PersistedState, error) {
	var states []*PersistedState
	for _, state := range p.states {
		states = append(states, state)
	}
	return states, nil
}

func (p *inmemoryStatePersister) DeleteState(_ context.Context, executionId uuid.UUID) error {
	delete(p.states, executionId)
	return nil
}
