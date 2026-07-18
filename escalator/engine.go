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

// RuleEngine maintains internal state (like Heartrate) and deterministically
// evaluates rules to emit events.
type RuleEngine struct {
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
}

// NewRuleEngine initializes a new engine with resting defaults.
func NewRuleEngine() *RuleEngine {
	now := time.Now()
	return &RuleEngine{
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
}

// OnResponse is called each time Lyra sends a reply.
// It drains mental energy by 10 and slightly drops the heartrate.
func (e *RuleEngine) OnResponse() {
	e.MentalEnergy -= 10.0
	if e.MentalEnergy < 0 {
		e.MentalEnergy = 0
	}
	// Each response costs a little HR too (cognitive effort)
	e.Heartrate -= 2.0
	if e.Heartrate < 40.0 {
		e.Heartrate = 40.0
	}
}
