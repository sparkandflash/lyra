# MSRPEngine Roadmap

This document outlines the evolutionary steps for the Mind State Reactive Personality Engine (MSRPEngine). Rather than attempting to build everything at once, development is broken down into distinct, focused versions. The goal is to build a solid framework for experimenting with persistent cognition.

## Immediate Interface Goals
Now that the core MSRPEngine is cleanly decoupled and isolated in its own terminal-app architecture, future development will expand into new interfaces:
*   **Discord Bot Integration:** Exposing the engine as an active participant in Discord servers.
*   **Browser-Based Web App:** A rich HTML/CSS/JS interface running locally or deployed to the web.
*   **Messaging Logic Adjustments:** Lyra will think before replying to the user, or decide not to respond. She will take her time, and her replies need not be instantaneous.

---

## V3: Memory Rewrite
**Goal: Better memory.**

The memory system is the foundation. Everything else (personality, dreams) sits on top of it. If memory isn't solid, personality will be learning from unstable data.

*   **Timeline:** Organizing episodes chronologically first, then by emotional scores.
*   **Facts:** Retaining all information from the interface history without excessive pre-filtering.
*   **Linker:** An idle process that links facts with other facts based on new context. Instead of independent episodes, memories become a semantic graph.
*   **Retriever:** An active process triggered during response generation. It finds the nearest fact, expands the neighborhood via linked episodes, and injects a richer context into the active window.
*   **Wikipedia / External Search:** A basic tool allowing the responder to seek info outside its context.
*   **Provenance / Origin Tracking:** Every fact must know its origin (e.g., Interface History, Wikipedia, Dream, Personality Inference). This allows the Retriever to distinguish between "I experienced this" and "I read this."
*   **Memory Scaling & Concurrency:** As the memory graph grows, the system will process memories in isolated "chunks" and rotate them. Idle methods (like linking and model creation) will run concurrently across different chunks. The architecture will also support spinning up multiple parallel `Responder + Context` instances.
*   **Memory Splitting (Objective vs. Subjective):** Memories will be structurally divided into two distinct categories:
    *   **Objective Memory:** Immutable, raw records of reality (e.g., interaction history, factual summaries).
    *   **Subjective Memory:** Fluid, engine-generated interpretations (e.g., processed memories, semantic linking, and Candidate Models).
---

## V4: Personality Architecture
**Goal: Can personality emerge?**

Once the V3 memory graph is populated with stable real-world data, the engine can begin trait extraction.

*   **Fixed Core + Mutable Layer:** Implementing a short, concrete core identity (values logic, earned warmth) that never changes, combined with a mutable trait layer that accumulates and evolves on top of it.
*   **Trait Acquisition Sources:** Traits picked up from interactions across *multiple* users and external sources.
*   **The Soul Score (Trait Retention Gate):** Gating trait retention on the *intensity* of mindstate movement (meaningful spikes in Serotonin, Oxytocin, Cortisol, or Attention), not just positive valence.
*   **Asymmetric Friction:** Making it easier to add a new trait than to remove an established one, ensuring personality development compounds.
*   **Pipeline:** `Facts → Trait extraction → Short-term personality → Stable personality → Identity`

---

## V5: Dreaming / Inner Thought
**Goal: Can self-reasoning improve the personality?**

An autonomous reasoning agent functioning during deep sleep/hibernation states.

*   **Inner Thought Function:** The LLM talks to itself, simulating memory queries and discussing past events.
*   **Time Travel & Analysis:** The ability to travel through the memory timeline to analyze and reason about past interactions.
*   **REM Sleep Constraints:** Dreams shouldn't rewrite history directly. They are limited to observations ("I noticed...", "Maybe..."). Hard decisions must still go through the Developer/Personality pipeline to prevent runaway reality shifts.
*   **Pipeline:** `Memory → Reasoning → Self discussion → New conclusions → Memory updates`

---

## V6: The Hypothesis Engine (Imagination & Validation)
**Goal: Can a persistent memory system generate and refine abstract beliefs over time?**

Moving beyond simple memory compression, the engine will act as a scientific observer, generating hypotheses about the world or the user and validating them against future reality. This harnesses the LLM's tendency to "hallucinate" by reframing it as **inference** and **imagination**.

*   **Candidate Models (Hypotheses):** Combined with the Inner Monologue idle method, this process doesn't run constantly. Instead, it triggers whenever a cluster of Episodes reaches a sufficient density of linking. The engine observes these linked patterns (e.g., "User likes backend," "User likes Go") and imagines a Candidate Model (e.g., "User values simplicity"). 
*   **Confidence Tiers:**
    *   `Episode:` 100% observed reality (Immutable).
    *   `Confirmed Model:` 95% confidence (Validated patterns).
    *   `Candidate Model:` 20% confidence (Unproven hypothesis).
    *   *Note:* The Responder should never present Candidates as absolute truth, only as internal possibilities.
*   **Validation Loop:** As new interface events occur, the engine tests its Candidate Models against new Episodes. If reality aligns, confidence increases until it becomes a Confirmed Model. 
*   **The Golden Rule ("Reality Always Wins"):** Observed memories are immutable. Models summarize observations. Candidates predict reality. If a user explicitly contradicts a Candidate Model, it is instantly rejected and deleted. The engine cannot drift into delusion because reality has the final vote.
*   **Pipeline:** `Interface History → Episodes → Episode Linking → Models → Model Candidates`
