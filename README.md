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
Now that the core MSRPEngine is cleanly decoupled and isolated in its own terminal-app architecture, future development will expand into new interfaces:
*   **Discord Bot Integration:** Exposing the engine as an active participant in Discord servers.
*   **Browser-Based Web App:** A rich HTML/CSS/JS interface running locally or deployed to the web.
*   **Messaging Logic Adjustments:** Lyra will think before replying to the user, or decide not to respond. She will take her time, and her replies need not be instantaneous.
*   **Lyra's Soul / Personality Architecture (Future Exploration):**
    *   **Fixed Core + Mutable Layer:** Implementing a short, concrete core identity (values logic, earned warmth) that never changes, combined with a mutable trait layer that accumulates and evolves on top of it.
    *   **Trait Acquisition Sources:** Traits picked up from interactions across *multiple* users (so no single person dominates) and through wiki/web searches during introspection to give her external interests.
    *   **The Soul Score (Trait Retention Gate):** Gating trait retention on the *intensity* of mindstate movement (meaningful spikes in Serotonin, Oxytocin, Cortisol, or Attention), not just positive valence. This resolves the sycophancy trap by rewarding "pissed the user off" equally to "pleased the user."
    *   **Asymmetric Friction:** Making it easier to add a new trait than to remove an established one, ensuring personality development compounds rather than flip-flops on noisy single-episode data.
    *   **Open Risks to Address:** 
        *   Mitigating "intensity-maximization" reward hacking (where chaos and depth look identical to an intensity-only gate).
        *   Explicitly designing attribution/decay for multi-user trait pickups to prevent raw conversation volume from a single user recreating the mirror problem.
        *   Implementing a decay-on-long-absence exception so early traits don't gain disproportionate permanence simply due to timing.
        *   Logging trait provenance (user, timestamp, persistence) from day one to diagnose whether the multi-user "natural pattern formation" succeeds or produces mush.
