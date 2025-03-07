package transition

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"strings"
)

// Transition is a struct, embed it in your struct to enable state machine for the struct
type Transition struct {
	State           string
}

// SetState set state to Stater, just set, won't save it into database
func (transition *Transition) SetState(name string) {
	transition.State = name
}

// GetState get current state from
func (transition Transition) GetState() string {
	return transition.State
}

// Stater is a interface including methods `GetState`, `SetState`
type Stater interface {
	SetState(name string)
	GetState() string
}

// New initialize a new StateMachine that hold states, events definitions
func New(value interface{}) *StateMachine {
	return &StateMachine{
		states: map[string]*State{},
		events: map[string]*Event{},
	}
}

// StateMachine a struct that hold states, events definitions
type StateMachine struct {
	initialState string
	states       map[string]*State
	events       map[string]*Event
}

func (sm *StateMachine) EventAllowed(val Stater, eventName string) bool{
	event := sm.events[eventName]
	for _, transition := range event.transitions {
		if len(transition.froms) > 0 {
			for _, fromState := range transition.froms {
				if strings.Compare(fromState, val.GetState()) == 0 {
					return true
				}
			}
		}
	}
	return false
}

// Initial define the initial state
func (sm *StateMachine) Initial(name string) *StateMachine {
	sm.initialState = name
	return sm
}

// State define a state
func (sm *StateMachine) State(name string) *State {
	state := &State{Name: name}
	sm.states[name] = state
	return state
}

// Event define an event
func (sm *StateMachine) Event(name string) *Event {
	event := &Event{Name: name}
	sm.events[name] = event
	return event
}

// Trigger trigger an event
func (sm *StateMachine) Trigger(name string, value Stater, tx *gorm.DB, notes ...string) error {
	var (
		stateWas = value.GetState()
	)

	if stateWas == "" {
		stateWas = sm.initialState
		value.SetState(sm.initialState)
	}

	if event := sm.events[name]; event != nil {
		var matchedTransitions []*EventTransition
		for _, transition := range event.transitions {
			var validFrom = len(transition.froms) == 0
			if len(transition.froms) > 0 {
				for _, from := range transition.froms {
					if from == stateWas {
						validFrom = true
					}
				}
			}

			if validFrom {
				matchedTransitions = append(matchedTransitions, transition)
			}
		}

		if len(matchedTransitions) == 1 {
			transition := matchedTransitions[0]

			// State: exit
			if state, ok := sm.states[stateWas]; ok {
				for _, exit := range state.exits {
					if err := exit(value, tx); err != nil {
						return err
					}
				}
			}

			// Transition: before
			for _, before := range transition.befores {
				if err := before(value, tx); err != nil {
					return err
				}
			}

			value.SetState(transition.to)

			// State: enter
			if state, ok := sm.states[transition.to]; ok {
				for _, enter := range state.enters {
					if err := enter(value, tx); err != nil {
						value.SetState(stateWas)
						return err
					}
				}
			}

			// Transition: after
			for _, after := range transition.afters {
				if err := after(value, tx); err != nil {
					value.SetState(stateWas)
					return err
				}
			}

			if tx != nil {
				return tx.Save(value).Error
			}

		}
	}
	return fmt.Errorf("failed to perform event %s from state %s", name, stateWas)
}

// State contains State information, including enter, exit hooks
type State struct {
	Name   string
	enters []func(value interface{}, tx *gorm.DB) error
	exits  []func(value interface{}, tx *gorm.DB) error
}

// Enter register an enter hook for State
func (state *State) Enter(fc func(value interface{}, tx *gorm.DB) error) *State {
	state.enters = append(state.enters, fc)
	return state
}

// Exit register an exit hook for State
func (state *State) Exit(fc func(value interface{}, tx *gorm.DB) error) *State {
	state.exits = append(state.exits, fc)
	return state
}

// Event contains Event information, including transition hooks
type Event struct {
	Name        string
	transitions []*EventTransition
}

// To define EventTransition of go to a state
func (event *Event) To(name string) *EventTransition {
	transition := &EventTransition{to: name}
	event.transitions = append(event.transitions, transition)
	return transition
}

// EventTransition hold event's to/froms states, also including befores, afters hooks
type EventTransition struct {
	to      string
	froms   []string
	befores []func(value interface{}, tx *gorm.DB) error
	afters  []func(value interface{}, tx *gorm.DB) error
}

// From used to define from states
func (transition *EventTransition) From(states ...string) *EventTransition {
	transition.froms = states
	return transition
}

// Before register before hooks
func (transition *EventTransition) Before(fc func(value interface{}, tx *gorm.DB) error) *EventTransition {
	transition.befores = append(transition.befores, fc)
	return transition
}

// After register after hooks
func (transition *EventTransition) After(fc func(value interface{}, tx *gorm.DB) error) *EventTransition {
	transition.afters = append(transition.afters, fc)
	return transition
}
