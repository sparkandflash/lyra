# MSRPEngine Roadmap

This document outlines the evolutionary steps for the Mind State Reactive Personality Engine (MSRPEngine). Development is broken down into distinct, focused versions aimed at building a solid framework for experimenting with persistent cognition.

---

## V3: The Asynchronous Mind (Async Interface + Memory + Tools)
**Goal: Break the Chatbot Paradigm.**

V3 decouples the engine from the rigid request/response cycle, turning the LLM into a persistent simulated mind that thinks at its own pace.

*   **Continuous Thought Loop & Context Crawler:** The engine runs a continuous, async background loop. The Context Crawler searches memory based on the LLM's *current internal thought state*, not just user input.
*   **The Mode Switch (Convergence vs. Wandering):** When a user message is queued, the crawler seeks highly relevant facts to ground the LLM (Convergence). When alone, the crawler drifts into under-explored tangents (Wandering/Divergence).
*   **Slower Frame Rate & Energy Economics:** To survive free-tier APIs, the engine operates slower than a human. Loop frequency is modulated by Reactor scores. A strict Mental Energy economy acts as a circuit breaker against runaway token spikes.
*   **Foundational Memory Rewrite:**
    *   **Objective vs. Subjective:** Separating raw interaction history from engine-generated interpretations.
    *   **Provenance:** Every fact tracks its origin (Interface, Dream, Wikipedia) so the engine knows *how* it knows something.
*   **Tools Expansion:** Giving the async mind the ability to reach out (e.g., Wikipedia / External Search) during its thought cycles.

---

## V4: The Hypothesis Engine (Model Formation, Validation & Personality)
**Goal: Move from storing facts to building beliefs.**

Once the async mind can think independently, it needs to form hypotheses about the world, the user, and itself using heavy deterministic **Idle Methods** during rest periods.

*   **Candidate Models (Hypotheses):** When memory clusters reach sufficient density, the engine imagines a Candidate Model (e.g., "User values simplicity").
*   **Confidence Tiers & Validation Loop:** `Episode` (100%) -> `Candidate Model` (20%) -> `Confirmed Model` (95%). The engine tests Candidates against new realities over time. 
*   **The Golden Rule:** "Reality always wins." If the user contradicts a Candidate Model, it is rejected.
*   **Pattern Completion Instinct:** An idle method that traverses the context graph and naturally generates questions to fill in missing gaps in its models.
*   **Personality Emergence (Models of Self):** Applying model-formation inward.
    *   **Fixed Core + Mutable Layer:** A concrete core identity that never changes, with a mutable trait layer that evolves.
    *   **The Soul Score:** Gating trait retention based on the intensity of Reactor mindstate spikes, ensuring her personality development compounds organically.

---

## V5: Dreaming & Inner Thought
**Goal: Can self-reasoning refine the context map?**

An autonomous reasoning agent functioning specifically during deep sleep/hibernation states to refine the memory graph.

*   **Time Travel & Analysis:** The ability to traverse the timeline to analyze and reason about past interactions far outside the current context window.
*   **Self-Discussion:** The LLM talks to itself in a closed loop, simulating memory queries and discussing past events to find new connections.
*   **REM Sleep Constraints:** Dreams cannot rewrite history directly. They are limited to subjective observations ("I noticed...", "Maybe..."). Hard conclusions must pass through the V4 Validation Loop to prevent runaway reality shifts/delusions.
