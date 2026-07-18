package escalator

import (
	"time"
)

// EventType represents a declarative action decided by the Rule Engine.
type EventType string

const (
	EventNothing          EventType = "NOTHING"
	EventConsolidate      EventType = "CONSOLIDATE"
	EventReflect          EventType = "REFLECT"
	EventProactiveMessage EventType = "PROACTIVE_MESSAGE"
	EventIntrospect       EventType = "INTROSPECT"
	EventEnterTempSleep   EventType = "ENTER_TEMP_SLEEP"
	EventEnterTrueSleep   EventType = "ENTER_TRUE_SLEEP"
)

// RuleEngine is the interface for evaluating state and managing internal variables like heartrate and mental energy.
type RuleEngine interface {
	UpdateHeartrate(mindState string)
	OnUserMessage(mindState string)
	EvaluateState(mindState string, hasUnconsolidatedMessages bool) EventType
	AcknowledgeEvent(evt EventType)
	OnResponse()
	ConsumeEnergy(amount float64)

	GetHeartrate() float64
	GetMentalEnergy() float64
	SetMentalEnergy(energy float64)
	GetCurrentSleepMode() int
	SetSleepMode(mode int)
}

// DefaultRuleEngine maintains internal state (like Heartrate) and deterministically
// evaluates rules to emit events based on an embedded YAML rules file.
type DefaultRuleEngine struct {
	// Internal State
	Heartrate              float64
	MentalEnergy           float64 // 0–100. Drains per response, regens at resting HR.
	MovingAverageUserDelay time.Duration
	CurrentSleepMode       int     // 0 = Awake, 1 = TempSleep, 2 = TrueSleep

	// Timestamps
	LastUserMessage      time.Time
	LastAssistantMessage time.Time
	LastConsolidation    time.Time
	LastReflection       time.Time
	LastIntrospection    time.Time
	LastProactiveMessage time.Time

	compiledModifiers []CompiledModifier
	compiledRules     []CompiledRule
}

type CompiledModifier struct {
	Condition interface{} // *vm.Program
	Effect    interface{} // *vm.Program
}

type CompiledRule struct {
	Name      string
	Condition interface{} // *vm.Program
	Action    EventType
	Priority  int
}

// NewRuleEngine initializes a new engine with resting defaults.
func NewRuleEngine() RuleEngine {
	now := time.Now()
	engine := &DefaultRuleEngine{
		Heartrate:              70.0,
		MentalEnergy:           100.0,
		MovingAverageUserDelay: 10 * time.Second, // Default starting assumption
		CurrentSleepMode:       0,
		LastUserMessage:        now,
		LastAssistantMessage:   now,
		LastConsolidation:      now,
		LastReflection:         now,
		LastIntrospection:      now,
		LastProactiveMessage:   now,
	}
	engine.initRules()
	return engine
}

func (e *DefaultRuleEngine) GetHeartrate() float64 {
	return e.Heartrate
}

func (e *DefaultRuleEngine) GetMentalEnergy() float64 {
	return e.MentalEnergy
}

func (e *DefaultRuleEngine) SetMentalEnergy(energy float64) {
	e.MentalEnergy = energy
}

func (e *DefaultRuleEngine) GetCurrentSleepMode() int {
	return e.CurrentSleepMode
}

func (e *DefaultRuleEngine) SetSleepMode(mode int) {
	e.CurrentSleepMode = mode
}

// ConsumeEnergy deducts mental energy and clamps to 0.
func (e *DefaultRuleEngine) ConsumeEnergy(amount float64) {
	e.MentalEnergy -= amount
	if e.MentalEnergy < 0 {
		e.MentalEnergy = 0
	}
}

// OnResponse is called each time Lyra sends a reply.
// It drains mental energy by 10 and slightly drops the heartrate.
func (e *DefaultRuleEngine) OnResponse() {
	e.ConsumeEnergy(10.0)
	// Each response costs a little HR too (cognitive effort)
	e.Heartrate -= 2.0
	if e.Heartrate < 40.0 {
		e.Heartrate = 40.0
	}
}
