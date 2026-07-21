# MSRPEngine

![AI Memory Architecture](docs/ai-mem.png)

Welcome to **MSRPEngine** (Mind State Reactive Personality Engine)—an experimental project designed to decouple the concept of an AI's "Subconscious System" from its "Personality". 

This architecture allows developers to create deeply stateful, emotionally intelligent companions. The underlying engine runs continuously in the background, managing complex memory structures, evaluating emotional context, and initiating proactive actions, while the specific "Persona" is seamlessly loaded on top as the conversational interface.

## Core Philosophy

Traditional LLMs are stateless request-response machines. This engine transforms an LLM into a living entity by wrapping it in an autonomous event loop.

*   **The System (Subconscious):** Handles background tasks like memory consolidation, deep introspection, emotional state tracking, and proactive event triggers (e.g., deciding to speak first when the user has been silent).
*   **The Persona (Conscious):** The actual identity interacting with the user. The Persona is utterly unaware of the background architecture; it simply receives a massive context injection (its current "Mindstate", energy levels, and recalled episodic memories) right before it speaks.

## Engine Features

### 1. Biological "Mindstate" Tracking
The System continuously monitors five indicators during conversation:
*   `Model Attention`
*   `User Attention`
*   `Serotonin` (Happiness / Sadness)
*   `Oxytocin` (Trust / Fear)
*   `Cortisol` (Stress / Relaxation)

These biological states dictate the engine's behavior. A high Cortisol score triggers the Persona to act panicked or stressed, while low Oxytocin triggers defensiveness. Crucially, these hormones act as multipliers for the engine's physical exhaustion.

### 2. Dual-Memory System
*   **Short-Term Memory (STM):** A highly constrained, rolling buffer of the immediate conversation.
*   **Long-Term Episodic Memory:** The System runs background "Consolidation" agents that read raw STM logs, summarize them, extract keywords, and bake them into permanent JSON "episodes" stored on disk. When similar topics arise in the future, a "Reflector" agent dynamically fetches relevant past episodes and injects them into the Persona's context window.

### 3. Biological Rhythms (Escalator)
The engine simulates fatigue and arousal using a real-time energy pool:
*   **Mental Energy:** A 1000-point pool that slowly depletes while the Persona is active. 
*   **Hormonal Drain:** Experiencing extreme emotions (high Cortisol, low Oxytocin) dramatically accelerates the energy drain rate.
*   **Exhaustion / Sleep:** If energy drops critically low (below 400), the engine physically overrides the Persona, forcing it to ignore the user with a system-level "no response" until it enters an idle or hibernation sleep cycle to naturally regenerate.

### 4. Portable Directories
The engine is self-contained within a portable directory containing the executable and its `.bin/` sidecar ML engine. You can compile a specific Persona and run it anywhere on your computer—it automatically spins up its own isolated context folders and configuration right next to itself.

## Getting Started

To dive into the technical details, configure the API keys, and build your own portable Persona bots, head over to the [Terminal App Documentation](terminal-app/README.md).

## Roadmap

The detailed evolutionary plan for MSRPEngine, including the upcoming memory rewrite (V3), personality emergence (V4), and dreaming functionality (V5), has been moved to its own document. 

Please see the [ROADMAP.md](ROADMAP.md) for full details.
