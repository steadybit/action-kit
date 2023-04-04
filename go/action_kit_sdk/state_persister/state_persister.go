// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package state_persister

import (
	"context"
	"github.com/google/uuid"
)

type PersistedState struct {
	ExecutionId uuid.UUID
	ActionId    string
	State       interface{}
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
