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
	GetEnergyDrainRate() float64
	CheckBiologicalEvents(newMindState string) string
}

// DefaultRuleEngine maintains internal state (like Heartrate) and deterministically
// evaluates rules to emit events based on an embedded YAML rules file.
type DefaultRuleEngine struct {
	Heartrate              float64
	MentalEnergy           float64 // 0–1000.
	CurrentDrainRate       float64
	MovingAverageUserDelay time.Duration
	CurrentSleepMode       int     // 0 = Awake, 1 = TempSleep, 2 = TrueSleep

	// Timestamps
	LastUserMessage      time.Time
	LastAssistantMessage time.Time
	LastConsolidation    time.Time
	LastReflection       time.Time
	LastIntrospection    time.Time
	LastProactiveMessage time.Time

	// Previous Biological State for Spikes
	prevEnergy          float64
	prevSE              float64
	prevOX              float64
	prevCO              float64
	biologicalStateInit bool

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

func NewRuleEngine() RuleEngine {
	now := time.Now()
	engine := &DefaultRuleEngine{
		Heartrate:              70.0,
		MentalEnergy:           800.0,
		CurrentDrainRate:       10.0,
		MovingAverageUserDelay: 10 * time.Second, // Default starting assumption
		CurrentSleepMode:       2,                // Default to Hibernation (True Sleep) on startup
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
	// Dynamically calculate HR based on the current drain rate
	hr := 60.0 + (e.CurrentDrainRate * 2.5) // Example mapping: drain of 10 = 85 BPM, 40 = 160 BPM
	if hr > 180 { return 180 }
	if hr < 40 { return 40 }
	return hr
}

func (e *DefaultRuleEngine) GetEnergyDrainRate() float64 {
	return e.CurrentDrainRate
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
// It drains mental energy dynamically based on the current mind state.
func (e *DefaultRuleEngine) OnResponse() {
	// The actual dynamic drain calculation happens in UpdateHeartrate/EvaluateState now,
	// but we apply the current drain rate per response.
	e.ConsumeEnergy(e.CurrentDrainRate)
}
