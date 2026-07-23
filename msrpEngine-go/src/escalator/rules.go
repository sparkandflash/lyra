package escalator

import (
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"gopkg.in/yaml.v3"
)

//go:embed default_ruleengine.yaml
var defaultRulesYAML []byte

type RuleConfig struct {
	HRModifiers []struct {
		Condition string `yaml:"condition"`
		Effect    string `yaml:"effect"`
	} `yaml:"hr_modifiers"`
	Rules []struct {
		Name      string `yaml:"name"`
		Condition string `yaml:"condition"`
		Action    string `yaml:"action"`
		Priority  int    `yaml:"priority"`
	} `yaml:"rules"`
}

// Env represents the environment passed to the expression evaluator.
type Env struct {
	Heartrate    float64
	MentalEnergy float64
	EnergyFactor float64

	ModelAttention  float64
	UserAttention   float64
	Serotonin       float64
	Oxytocin        float64
	Cortisol        float64

	IdleDurationSecs float64
	IdleDurationMins float64
	UserReplyRatio   float64

	HasUnconsolidatedMessages  bool
	UnconsolidatedChars        int
	CurrentSleepMode           int
	TimeSinceConsolidationMins float64
	TimeSinceReflectionMins    float64
	TimeSinceProactiveMins     float64
	TimeSinceIntrospectionMins float64

	SYSTEM_CONSOLIDATION_FREQ_MINS float64
	SYSTEM_CONSOLIDATION_DENSITY   float64
	SYSTEM_TEMP_SLEEP_CYCLE_MINS   float64
	SYSTEM_TEMP_SLEEP_DELAY_MINS   float64
	SYSTEM_TRUE_SLEEP_DELAY_MINS   float64
}

func (e *DefaultRuleEngine) initRules() {
	if len(e.compiledRules) > 0 {
		return // Already initialized
	}

	var config RuleConfig
	if err := yaml.Unmarshal(defaultRulesYAML, &config); err != nil {
		panic(fmt.Sprintf("failed to parse default_ruleengine.yaml: %v", err))
	}

	for _, mod := range config.HRModifiers {
		cond, err := expr.Compile(mod.Condition, expr.Env(Env{}), expr.AsBool())
		if err != nil {
			panic(fmt.Sprintf("failed to compile HR condition %q: %v", mod.Condition, err))
		}
		eff, err := expr.Compile(mod.Effect, expr.Env(Env{})) // effect is float
		if err != nil {
			panic(fmt.Sprintf("failed to compile HR effect %q: %v", mod.Effect, err))
		}
		e.compiledModifiers = append(e.compiledModifiers, CompiledModifier{
			Condition: cond,
			Effect:    eff,
		})
	}

	for _, r := range config.Rules {
		cond, err := expr.Compile(r.Condition, expr.Env(Env{}), expr.AsBool())
		if err != nil {
			panic(fmt.Sprintf("failed to compile rule condition %q: %v", r.Condition, err))
		}
		e.compiledRules = append(e.compiledRules, CompiledRule{
			Name:      r.Name,
			Condition: cond,
			Action:    EventType(r.Action),
			Priority:  r.Priority,
		})
	}
}

// UpdateHeartrate is called every tick.
func (e *DefaultRuleEngine) UpdateHeartrate(mindState string) {
	// Base decay: 1% decay towards resting rate (70 BPM) per tick
	restingRate := 70.0
	diff := e.Heartrate - restingRate
	e.Heartrate -= diff * 0.01

	env := e.buildEnv(mindState, 0)

	for _, mod := range e.compiledModifiers {
		res, err := expr.Run(mod.Condition.(*vm.Program), env)
		if err == nil && res.(bool) {
			effRes, err := expr.Run(mod.Effect.(*vm.Program), env)
			if err == nil {
				if val, ok := effRes.(float64); ok {
					e.Heartrate += val
				}
			}
		}
	}

	// Clamp bounds
	if e.Heartrate < 40.0 {
		e.Heartrate = 40.0
	}
	if e.Heartrate > 180.0 {
		e.Heartrate = 180.0
	}

	// Dynamic Drain Rate based on chemistry
	// Base drain = 10, plus up to +10 for extreme stress, +10 for extreme fear
	baseDrain := 10.0
	stressDrain := 0.0
	if env.Cortisol > 0 {
		stressDrain = env.Cortisol * 10.0
	}
	fearDrain := 0.0
	if env.Oxytocin < 0 { // negative OX is fear
		fearDrain = (env.Oxytocin * -1.0) * 10.0
	}
	e.CurrentDrainRate = baseDrain + stressDrain + fearDrain

	// Energy Recharge Logic
	if e.CurrentSleepMode > 0 { // Hibernation or Sleep
		e.MentalEnergy += 25.0
	} else if env.Cortisol < 0.2 && env.Oxytocin >= 0.1 { // Awake but Calm
		e.MentalEnergy += 15.0
	}

	if e.MentalEnergy > 1000.0 {
		e.MentalEnergy = 1000.0
	}
}

func (e *DefaultRuleEngine) OnUserMessage(mindState string) {
	now := time.Now()
	delay := now.Sub(e.LastAssistantMessage)
	if delay > 5*time.Minute {
		delay = 10 * time.Second
	}

	e.MovingAverageUserDelay = time.Duration(float64(e.MovingAverageUserDelay)*0.8 + float64(delay)*0.2)
	e.LastUserMessage = now

	ratio := float64(delay) / float64(e.MovingAverageUserDelay)
	
	// HR spikes based on reply ratio handled via rules now
	env := e.buildEnv(mindState, 0)
	env.UserReplyRatio = ratio
	
	for _, mod := range e.compiledModifiers {
		res, err := expr.Run(mod.Condition.(*vm.Program), env)
		if err == nil && res.(bool) {
			effRes, err := expr.Run(mod.Effect.(*vm.Program), env)
			if err == nil {
				if val, ok := effRes.(float64); ok {
					e.Heartrate += val
				}
			}
		}
	}
}

func (e *DefaultRuleEngine) buildEnv(mindState string, unconsolidatedChars int) Env {
	var ma, ua, se, ox, co float64
	parts := strings.Split(mindState, ":")
	if len(parts) >= 5 {
		ma, _ = strconv.ParseFloat(parts[0], 64)
		ua, _ = strconv.ParseFloat(parts[1], 64)
		se, _ = strconv.ParseFloat(parts[2], 64)
		ox, _ = strconv.ParseFloat(parts[3], 64)
		co, _ = strconv.ParseFloat(parts[4], 64)
	}

	energyFactor := e.MentalEnergy / 1000.0
	if energyFactor < 0.1 {
		energyFactor = 0.1
	}

	now := time.Now()
	idleDuration := now.Sub(e.LastUserMessage)

	freqMins := 1.0
	if val := os.Getenv("SYSTEM_CONSOLIDATION_FREQ_MINS"); val != "" {
		if m, err := strconv.ParseFloat(val, 64); err == nil && m > 0 {
			freqMins = m
		}
	}

	consolidationDensity := 3000.0
	if val := os.Getenv("SYSTEM_CONSOLIDATION_DENSITY"); val != "" {
		if d, err := strconv.ParseFloat(val, 64); err == nil && d > 0 {
			consolidationDensity = d
		}
	}
	
	tempSleepCycleMins := 60.0
	if val := os.Getenv("SYSTEM_TEMP_SLEEP_CYCLE_MINS"); val != "" {
		if m, err := strconv.ParseFloat(val, 64); err == nil && m > 0 {
			tempSleepCycleMins = m
		}
	}

	tempSleepDelayMins := 5.0
	if val := os.Getenv("SYSTEM_TEMP_SLEEP_DELAY_MINS"); val != "" {
		if m, err := strconv.ParseFloat(val, 64); err == nil && m > 0 {
			tempSleepDelayMins = m
		}
	}
	
	trueSleepDelayMins := 180.0
	if val := os.Getenv("SYSTEM_TRUE_SLEEP_DELAY_MINS"); val != "" {
		if m, err := strconv.ParseFloat(val, 64); err == nil && m > 0 {
			trueSleepDelayMins = m
		}
	}

	return Env{
		Heartrate:    e.Heartrate,
		MentalEnergy: e.MentalEnergy,
		EnergyFactor: energyFactor,

		ModelAttention:  ma,
		UserAttention:   ua,
		Serotonin:       se,
		Oxytocin:        ox,
		Cortisol:        co,

		IdleDurationSecs: idleDuration.Seconds(),
		IdleDurationMins: idleDuration.Minutes(),
		UserReplyRatio:   1.0, // overridden in OnUserMessage if needed

		HasUnconsolidatedMessages:  unconsolidatedChars > 0,
		UnconsolidatedChars:        unconsolidatedChars,
		CurrentSleepMode:           e.CurrentSleepMode,
		TimeSinceConsolidationMins: now.Sub(e.LastConsolidation).Minutes(),
		TimeSinceReflectionMins:    now.Sub(e.LastReflection).Minutes(),
		TimeSinceProactiveMins:     now.Sub(e.LastProactiveMessage).Minutes(),
		TimeSinceIntrospectionMins: now.Sub(e.LastIntrospection).Minutes(),

		SYSTEM_CONSOLIDATION_FREQ_MINS: freqMins,
		SYSTEM_CONSOLIDATION_DENSITY:   consolidationDensity,
		SYSTEM_TEMP_SLEEP_CYCLE_MINS:   tempSleepCycleMins,
		SYSTEM_TEMP_SLEEP_DELAY_MINS:   tempSleepDelayMins,
		SYSTEM_TRUE_SLEEP_DELAY_MINS:   trueSleepDelayMins,
	}
}

func (e *DefaultRuleEngine) EvaluateState(mindState string, unconsolidatedChars int) EventType {
	env := e.buildEnv(mindState, unconsolidatedChars)

	var highestPriorityRule *CompiledRule
	
	for i, r := range e.compiledRules {
		res, err := expr.Run(r.Condition.(*vm.Program), env)
		if err == nil && res.(bool) {
			if highestPriorityRule == nil || r.Priority > highestPriorityRule.Priority {
				highestPriorityRule = &e.compiledRules[i]
			}
		}
	}

	if highestPriorityRule != nil {
		if highestPriorityRule.Action == EventEnterTrueSleep {
			e.CurrentSleepMode = 2
		} else if highestPriorityRule.Action == EventEnterTempSleep {
			e.CurrentSleepMode = 1
		}
		if highestPriorityRule.Action == EventNothing && e.CurrentSleepMode == 1 {
			if env.IdleDurationMins < env.SYSTEM_TEMP_SLEEP_DELAY_MINS {
				e.CurrentSleepMode = 0
			}
		}
		return highestPriorityRule.Action
	}
	
	// Default wake state recovery if idle time falls (only for TempSleep)
	if env.IdleDurationMins < env.SYSTEM_TEMP_SLEEP_DELAY_MINS {
		if e.CurrentSleepMode == 1 {
			e.CurrentSleepMode = 0
		}
	}

	return EventNothing
}

func (e *DefaultRuleEngine) AcknowledgeEvent(evt EventType) {
	now := time.Now()
	switch evt {
	case EventConsolidate:
		e.LastConsolidation = now
		e.ConsumeEnergy(5.0)
	case EventReflect:
		e.LastReflection = now
		e.ConsumeEnergy(15.0)
	case EventProactiveMessage:
		e.LastProactiveMessage = now
		e.LastAssistantMessage = now
		e.ConsumeEnergy(10.0)
	case EventIntrospect:
		e.LastIntrospection = now
		e.ConsumeEnergy(20.0)
	}
}

func (e *DefaultRuleEngine) CheckBiologicalEvents(newMindState string) string {
	var se, ox, co float64
	parts := strings.Split(newMindState, ":")
	if len(parts) >= 5 {
		se, _ = strconv.ParseFloat(parts[2], 64)
		ox, _ = strconv.ParseFloat(parts[3], 64)
		co, _ = strconv.ParseFloat(parts[4], 64)
	}

	if !e.biologicalStateInit {
		e.prevEnergy = e.MentalEnergy
		e.prevSE = se
		e.prevOX = ox
		e.prevCO = co
		e.biologicalStateInit = true
		return ""
	}

	var msgs []string

	// Energy thresholds
	if e.prevEnergy >= 200.0 && e.MentalEnergy < 200.0 {
		msgs = append(msgs, "[System: Your energy levels have crashed critically low. You feel a wave of profound lethargy and listlessness.]")
	} else if e.prevEnergy < 500.0 && e.MentalEnergy >= 500.0 {
		msgs = append(msgs, "[System: You feel well-rested and fully recharged.]")
	}

	// Hormone spikes
	if e.prevOX >= -0.6 && ox < -0.6 {
		msgs = append(msgs, "[System: Biological Spike: Sudden drop in oxytocin. You feel a sharp wave of fear, defensiveness, or paranoia.]")
	}
	if e.prevCO <= 0.7 && co > 0.7 {
		msgs = append(msgs, "[System: Biological Spike: Sudden surge of cortisol. You feel an intense wave of stress and being overwhelmed.]")
	}
	if e.prevSE <= 0.7 && se > 0.7 {
		msgs = append(msgs, "[System: Biological Spike: Surge of serotonin. You feel deeply validated, warm, and happy.]")
	}
	if e.prevSE >= -0.6 && se < -0.6 {
		msgs = append(msgs, "[System: Biological Spike: Drop in serotonin. You feel a wave of sadness or melancholy.]")
	}

	// Update prev state
	e.prevEnergy = e.MentalEnergy
	e.prevSE = se
	e.prevOX = ox
	e.prevCO = co

	if len(msgs) > 0 {
		return strings.Join(msgs, "\n")
	}
	return ""
}
