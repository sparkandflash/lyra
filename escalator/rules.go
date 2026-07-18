package escalator

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// UpdateHeartrate is called every tick. It decays HR towards resting (70)
// and applies spikes based on the current mindstate and user delay.
func (e *RuleEngine) UpdateHeartrate(mindState string) {
	// Base decay: 1% decay towards resting rate (70 BPM) per tick
	restingRate := 70.0
	diff := e.Heartrate - restingRate
	e.Heartrate -= diff * 0.01 // Decay 1% of the difference

	// Parse Mindstate (MA:NE:PE:UA)
	var ma, ne, pe, ua float64
	parts := strings.Split(mindState, ":")
	if len(parts) == 4 {
		ma, _ = strconv.ParseFloat(parts[0], 64)
		ne, _ = strconv.ParseFloat(parts[1], 64)
		pe, _ = strconv.ParseFloat(parts[2], 64)
		ua, _ = strconv.ParseFloat(parts[3], 64)
	}

	// Rule: Spike HR if Model Attention or User Attention is high
	if ma > 0.8 || ua > 0.8 {
		e.Heartrate += 2.0
	}

	// Rule: Spike HR for strong emotional conversations
	if ne > 0.7 || pe > 0.7 {
		e.Heartrate += 3.0
	}

	// Rule: Decrease HR if conversation is emotionally neutral and attention is low
	if ne < 0.2 && pe < 0.2 && ma < 0.4 && ua < 0.4 {
		e.Heartrate -= 1.0
	}

	// Rule: Time-based spikes
	// If the user suddenly replied much faster or slower than their moving average.
	// This is evaluated elsewhere when a message actually arrives (in OnUserMessage).
	// But long idle periods should slowly drop HR.
	idleDuration := time.Since(e.LastUserMessage)
	if idleDuration > 2*time.Minute {
		e.Heartrate -= 0.5 // Extra decay for inactivity
	}

	// Clamp bounds
	if e.Heartrate < 40.0 {
		e.Heartrate = 40.0
	}
	if e.Heartrate > 180.0 {
		e.Heartrate = 180.0
	}

	// Mental Energy regen: if HR is at or near resting (≤ 72), recover 10 energy per tick.
	// This simulates Lyra "recovering" during calm periods.
	restingThreshold := 72.0
	if e.Heartrate <= restingThreshold {
		e.MentalEnergy += 10.0
		if e.MentalEnergy > 100.0 {
			e.MentalEnergy = 100.0
		}
	}
}

// OnUserMessage updates moving average delay and applies immediate HR spikes.
func (e *RuleEngine) OnUserMessage(mindState string) {
	now := time.Now()
	delay := now.Sub(e.LastAssistantMessage)
	
	// If first message or very long gap, don't heavily skew average, just use 10s baseline
	if delay > 5*time.Minute {
		delay = 10 * time.Second
	}

	// Moving average calculation (alpha = 0.2)
	e.MovingAverageUserDelay = time.Duration(float64(e.MovingAverageUserDelay)*0.8 + float64(delay)*0.2)
	e.LastUserMessage = now

	// Compare current delay against average
	ratio := float64(delay) / float64(e.MovingAverageUserDelay)
	
	// Rule: User replied much faster (ratio < 0.3) or much slower (ratio > 3.0) than usual
	if ratio < 0.3 {
		e.Heartrate += 5.0
	} else if ratio > 3.0 {
		e.Heartrate += 5.0
	}
}

// EvaluateState checks all rules and determines if an event should be emitted.
// Returns the highest priority EventType.
func (e *RuleEngine) EvaluateState(mindState string, hasUnconsolidatedMessages bool) EventType {
	now := time.Now()

	// Parse Mindstate
	var ma float64
	parts := strings.Split(mindState, ":")
	if len(parts) == 4 {
		ma, _ = strconv.ParseFloat(parts[0], 64)
	}

	// 1. Consolidation (Timer driven, default 1 minute)
	freqMins := 1
	if val := os.Getenv("LYRA_CONSOLIDATION_FREQ_MINS"); val != "" {
		if m, err := strconv.Atoi(val); err == nil && m > 0 {
			freqMins = m
		}
	}
	if hasUnconsolidatedMessages && now.Sub(e.LastConsolidation) >= time.Duration(freqMins)*time.Minute {
		return EventConsolidate
	}

	// Time since last user activity
	idleDuration := now.Sub(e.LastUserMessage)

	// ── SLEEP STATE MACHINE ──────────────────────────────────────────
	if idleDuration >= 3*time.Hour {
		if e.CurrentSleepMode != 2 {
			e.CurrentSleepMode = 2
			return EventEnterTrueSleep
		}
	} else if idleDuration >= 5*time.Minute {
		if e.CurrentSleepMode != 1 {
			e.CurrentSleepMode = 1
			return EventEnterTempSleep
		}
	} else {
		e.CurrentSleepMode = 0 // Awake
	}

	// State 2: True Sleep (Hibernation) - Zero background tasks
	if e.CurrentSleepMode == 2 {
		return EventNothing
	}

	// State 1: Temp Sleep - Throttled tasks
	if e.CurrentSleepMode == 1 {
		// Allow one final consolidation to clean up memory
		if hasUnconsolidatedMessages {
			return EventConsolidate
		}

		// Throttled Introspection/Reflection
		tempSleepCycleMins := 60
		if val := os.Getenv("LYRA_TEMP_SLEEP_CYCLE_MINS"); val != "" {
			if m, err := strconv.Atoi(val); err == nil && m > 0 {
				tempSleepCycleMins = m
			}
		}
		if now.Sub(e.LastIntrospection) >= time.Duration(tempSleepCycleMins)*time.Minute {
			return EventIntrospect
		}
		
		return EventNothing
	}
	// ─────────────────────────────────────────────────────────────────

	// 2. Proactive Messaging
	// Rule: HR is high, MA is high, user inactive for 30s, and we haven't proactived recently (cooldown 1 min)
	if e.Heartrate > 90.0 && ma > 0.7 && idleDuration >= 30*time.Second {
		if now.Sub(e.LastProactiveMessage) >= 1*time.Minute {
			return EventProactiveMessage
		}
	}

	// 3. Reflection
	// Rule: High engagement but user inactive for 15s. (Cooldown 2 mins)
	if (e.Heartrate > 100.0 || ma > 0.8) && idleDuration >= 15*time.Second {
		if now.Sub(e.LastReflection) >= 2*time.Minute {
			return EventReflect
		}
	}

	// 4. Introspection
	// Rule: HR has declined to resting (< 75), user inactive for long time (2 mins). (Cooldown 5 mins)
	if e.Heartrate < 75.0 && idleDuration >= 2*time.Minute {
		if now.Sub(e.LastIntrospection) >= 5*time.Minute {
			return EventIntrospect
		}
	}

	return EventNothing
}

// AcknowledgeEvent updates the timestamp for when an event was successfully fired.
func (e *RuleEngine) AcknowledgeEvent(evt EventType) {
	now := time.Now()
	switch evt {
	case EventConsolidate:
		e.LastConsolidation = now
	case EventReflect:
		e.LastReflection = now
	case EventProactiveMessage:
		e.LastProactiveMessage = now
		e.LastAssistantMessage = now
	case EventIntrospect:
		e.LastIntrospection = now
	}
}
