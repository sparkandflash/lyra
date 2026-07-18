# Mind-State Roleplay Engine

Welcome to the **Mind-State Roleplay Engine**—an experimental project designed to decouple the concept of an AI's "Subconscious System" from its "Personality". 

This architecture allows developers to create deeply stateful, emotionally intelligent companions. The underlying engine runs continuously in the background, managing complex memory structures, evaluating emotional context, and initiating proactive actions, while the specific "Persona" is seamlessly loaded on top as the conversational interface.

## Core Philosophy

Traditional LLMs are stateless request-response machines. This engine transforms an LLM into a living entity by wrapping it in an autonomous event loop.

*   **The System (Subconscious):** Handles background tasks like memory consolidation, deep introspection, emotional state tracking, and proactive event triggers (e.g., deciding to speak first when the user has been silent).
*   **The Persona (Conscious):** The actual identity interacting with the user. The Persona is utterly unaware of the background architecture; it simply receives a massive context injection (its current "Mindstate", energy levels, and recalled episodic memories) right before it speaks.

## Engine Features

### 1. Emotional "Mindstate" Tracking
The System continuously monitors four emotional axes during conversation:
*   `Model Attention`
*   `Negative Emotion`
*   `Positive Emotion`
*   `User Attention`

These scores dictate the engine's behavior. A high negative emotion might trigger the Persona to respond more empathetically, while low mental energy forces the Persona to give short, exhausted responses.

### 2. Dual-Memory System
*   **Short-Term Memory (STM):** A highly constrained, rolling buffer of the immediate conversation.
*   **Long-Term Episodic Memory:** The System runs background "Consolidation" agents that read raw STM logs, summarize them, extract keywords, and bake them into permanent JSON "episodes" stored on disk. When similar topics arise in the future, a "Reflector" agent dynamically fetches relevant past episodes and injects them into the Persona's context window.

### 3. Biological Rhythms (Escalator)
The engine simulates fatigue and arousal:
*   **Heartrate:** Spikes during intense emotional exchanges, causing the System to process information faster.
*   **Mental Energy:** Slowly depletes while the Persona is active. If energy drops to zero, the engine forces the Persona to give terse, exhausted responses until the user allows it to "sleep" and recharge.

### 4. Portable Standalone Binaries
The engine is entirely self-contained. You can compile a specific Persona into a portable standalone binary. Once compiled, you can run the executable anywhere on your computer—it automatically spins up its own isolated context folders and configuration right next to itself.

## Getting Started

To dive into the technical details, configure the API keys, and build your own standalone Persona binaries, head over to the [Terminal App Documentation](terminal-app/README.md).
