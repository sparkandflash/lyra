# V3 Design Document: The Asynchronous Mind

## Paradigm Shift: "The Fly and the Human"
V3 fundamentally shifts the engine from a traditional "request -> response" chatbot into a **Persistent Simulated Mind**. 
To survive on free-tier APIs and accurately simulate cognition, the engine operates on a deliberately slower "frame rate" than human users. If Lyra is the human, the user is a fly. This means delayed responses are not a bug, but an accepted and expected feature. The user simply drops inputs into her environment, and she processes them at her own biological pace.

## The Continuous Thought Cycle
Instead of waiting for user input, the engine runs a continuous asynchronous loop:
1. **Context Formulation:** The current state of mind and immediate context is gathered.
2. **LLM Processing:** The LLM analyzes the context. It can choose to "answer" the context, autocomplete a thought, or ask an internal question.
3. **Context Crawler (Deterministic):** Based on the LLM's output, a deterministic crawler activates. It semantically searches the vector database using the LLM's *current internal thought state* as the query.
4. **Memory Injection & Rotation:** The crawler travels along memory links and timelines, capturing relevant material. This new material is injected back into the LLM's context window. To prevent bloat, old context is aggressively rotated out (FIFO) and replaced by the newly retrieved memory.
5. **Repeat:** The LLM processes the newly injected memory, continuing the chain of thought.

## Asynchronous User Interaction
The engine is always listening to the interface. The user can insert messages into the context at any time. However:
- The LLM does not immediately respond.
- The user's input simply becomes another piece of context for the Crawler and LLM to process.
- A combination of Reactor scores, the Rule Engine, and the LLM's internal state determines *if* and *when* to finally send a response to the user.

## Biological Frequency Control (The Reactor)
The frequency of the thought cycle is not static. It is dynamically controlled by the Reactor's biological mind scores:
- **Minimum Duration:** The absolute minimum time between API calls (e.g., 8 seconds) is set via environmental variables to respect rate limits. (This can be decreased later if rate limits allow).
- **Modulation:** If the engine experiences high Cortisol (stress/anxiety), the loop frequency spikes to simulate racing thoughts (e.g., hitting the 8-second minimum). If she is calm, the mind scores scale the delay up (e.g., 20+ seconds) for deep, slow contemplation.
- **Fluctuation:** One minute might see 3 API calls, while the next minute might see 6, creating a highly organic processing rhythm.

## Resting Periods & Idle Methods
The thought cycle does not run endlessly. The engine utilizes scheduled "Resting Periods" (Hibernation/True Sleep).
During these rests, the high-frequency thought cycle pauses, and heavy **Idle Methods** take over. These deterministic background processes are responsible for:
- Consolidating memories.
- Building, testing, and breaking Candidate Models (Hypotheses about the user and the world).
- Performing semantic linking across the memory graph.

## Energy Economics (Safeguard Against Token Spikes)
To prevent "racing thoughts" (High Cortisol) from causing an infinite high-frequency loop and draining API credits, the engine utilizes a strict **Mental Energy** economy:
- Every iteration of the thought loop consumes a specific amount of Mental Energy.
- If the thought loop is running at maximum frequency (e.g. 8-second pings due to panic or hyper-fixation), her energy pool will drain rapidly.
- As her energy depletes, the engine naturally throttles the loop frequency, forcing her to "slow down" and catch her breath, breaking the high-frequency spike.
- Only if her energy drops below a critical minimum threshold is she forcefully put into an "Exhaustion / Rest" state (Temp Sleep or True Sleep).
- This acts as a natural, biologically accurate circuit breaker. It guarantees that an autonomous, asynchronous LLM cannot runaway with API costs while the user is away from the computer.

## The Mode Switch: Convergence vs. Wandering
To mimic realistic human cognition, the Context Crawler toggles between two distinct retrieval modes, driven entirely by a simple, cheap signal: **The presence of a queued user message.**

### 1. Convergence Mode (User Message Present)
When a user message is waiting in the queue, the crawler seeks **convergence**. 
- It actively retrieves well-established, highly connected, and directly relevant memories.
- The goal is **grounding**: ensuring her thought cycle rapidly centers on the user's input so she can formulate a coherent, on-topic response.

### 2. Wandering Mode (No User Message)
When the user is absent, the crawler shifts into **novelty-seeking (wandering) mode**.
- It drifts toward under-explored, loosely connected, or novel memory nodes (the edges of her memory graph).
- This produces "mind-wandering" or day-dreaming, safely exploring tangents because no user is waiting for a direct answer.
- **Output Routing:** Thoughts generated in this mode are *not* automatically sent to the user. Instead, they are stored internally as candidate hypotheses, half-formed facts, or dream states. They only reach the user if the deterministic `ProactiveMessage` rule independently decides it's time to speak up.

### Clean Transitions (The Snap-to-Attention)
The transition between these two states is instantaneous. At the start of *every* thought cycle step, the engine checks for queued user input. If she is three hops deep into a tangential "wandering" thought chain and a user message suddenly arrives, the crawler immediately aborts the wander and snaps back into Convergence mode on the very next iteration. This ensures she doesn't respond to a direct question with a disorienting, tangent-flavored daydream.

## Neuromorphic Memory Graph (The "Neuron" Structure)
To move beyond basic vector storage and closer to biological Hebbian learning, V3 will restructure the database into a formal Graph Network of **Neurons**. 

Instead of flat JSON episodes, each memory is a distinct `Neuron` object containing:
- **Fact (Payload):** The semantic text or state variable.
- **Timestamp:** When it was created.
- **Synaptic Links:** Pointers to other related Neurons (representing edges in the graph).
- **Access Count:** An internal counter tracking exactly how many times the Context Crawler has traversed this specific node.
- **Metrics/Weight:** The biological "strength" of the memory, influenced by emotional mindstates at the time of creation or retrieval.

As the Context Crawler navigates this graph during the Async Thought Loop, Neurons that are frequently accessed together will mathematically strengthen their Synaptic Links, structurally altering the memory to prioritize frequently used pathways—simulating true biological memory!
