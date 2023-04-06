// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package state_persister

import (
	"context"
	"fmt"
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
	GetExecutionIds(ctx context.Context) ([]uuid.UUID, error)
	GetState(ctx context.Context, uuid uuid.UUID) (*PersistedState, error)
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

func (p *inmemoryStatePersister) GetExecutionIds(_ context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for id := range p.states {
		ids = append(ids, id)
	}
	return ids, nil
}

func (p *inmemoryStatePersister) GetState(_ context.Context, uuid uuid.UUID) (*PersistedState, error) {
	state, ok := p.states[uuid]
	if !ok {
		return nil, fmt.Errorf("state not found for execution id %s", uuid)
	}
	return state, nil
}

func (p *inmemoryStatePersister) DeleteState(_ context.Context, executionId uuid.UUID) error {
	delete(p.states, executionId)
	return nil
}
