# RFC-0001 — Vision & Product Philosophy

| | |
|---|---|
| **Status** | Draft — Proposed (seeking ratification) |
| **Authors** | Sahid Ayala (Chief Architect); Foundry Core |
| **Reviewers** | _(pending)_ |
| **Supersedes** | — |
| **Superseded by** | — |
| **Created** | 2026-06-29 |
| **Related** | `docs/ARCHITECTURE.md` (must conform to this RFC) |

> **What this document is.** RFC-0001 defines *what Foundry is and why it exists*. It is the constitution of the project. It does not describe mechanism — no languages, no modules, no APIs, no plugin systems. Those live in the architecture document and in later RFCs, and they are *downstream* of this one. When a future architectural decision conflicts with this RFC, this RFC wins until it is formally amended.
>
> **How to read it.** This is not a manifesto and not marketing. It is an argument, made so it can be argued with. Where I believe the founding brief is wrong, incomplete, or internally contradictory, I say so explicitly and mark it **⚠ Challenge** so it is easy to find and contest. A vision document that cannot be falsified is worthless; this one tries to be falsifiable.

---

## 1. Motivation

### 1.1 The situation we are responding to

The current generation of AI engineering tools has converged on a single shape: a human types intent into a conversation, a model emits code, the human accepts or rejects it, and the exchange is then thrown away. This shape is enormously useful and genuinely changed the daily work of engineering. It is also a **local maximum**, and we should be honest about why.

The unit of work in that shape is a *conversation*. Conversations have three properties that are fatal for engineering at scale:

1. **They do not compound.** The skill you applied to get a good result today is in your head, on that day, in that chat. Tomorrow you, or a teammate, start over. Nothing the organization learned is captured in a form the next task inherits.
2. **They are not accountable.** The output arrives without a durable record of *what knowledge informed it*, *what was checked*, and *why it was accepted*. When it later breaks, the reasoning is gone.
3. **Their quality is a function of operator prompting skill on a given day.** That is the definition of a craft that has not yet been engineered. We do not run production systems on "it depends how good the operator was feeling."

The industry's reflex has been to make the conversation better — better models, better prompts, better autocomplete, agents that take more steps unattended. All of that improves the *local* maximum. None of it changes the *shape*. A faster, smarter chat box is still a chat box, and its output still doesn't compound, still isn't accountable, and still rides on prompt craft.

### 1.2 The bet

Foundry is a bet that the durable value in AI-assisted engineering is **not** in generating code. Code generation is becoming a commodity that every model vendor will offer and improve faster than any independent project can. The durable value is in everything *around* generation: the engineering process that decides what to build, the knowledge that informs how, the verification that decides whether the result is trustworthy, and the auditable record of how a system reached its current state.

Put bluntly: **the model is the cheapest, most replaceable, and most rapidly commoditizing part of the stack.** Building a company or a platform whose value lives inside the model is building on someone else's land. Foundry's value must live in the layer the model vendors structurally cannot own: the organization's own engineering process and accumulated knowledge.

### 1.3 Why now, and why not just wait for better models

A reasonable skeptic asks: "Won't a sufficiently capable future model just do all of this internally, making the orchestration layer redundant?"

No — and the reason is not about model capability. It is about **trust and accountability**, which are properties of *process*, not of *intelligence*. A model can become arbitrarily good at producing a diff and still not answer the questions an engineering organization must answer to merge it: What knowledge was this decision based on? What was checked, and by what? Who approved it, and on what evidence? Can we reproduce how we got here? Can we audit it a year from now when it breaks?

These are not capability questions. A genius engineer who keeps no records, leaves no rationale, and cannot be audited is *not* trustworthy at organizational scale, no matter how good their code is. The orchestration layer exists to manufacture the trust that raw capability does not provide. Better models make that layer *more* valuable, not less, because they increase the volume of machine-produced work that an organization must be able to trust.

---

## 2. Goals

Foundry's goals, in priority order. Earlier goals constrain later ones.

1. **Make the engineering *process* the unit of work** — something nameable, versionable, shareable, testable, replayable, and improvable — rather than the conversation.
2. **Manufacture trust in machine-produced engineering work** — make AI output safe to depend on through provenance, deterministic verification, and human accountability, so the question "can I merge this?" has an evidence-based answer.
3. **Build durable engineering capital** — accumulate an organization's decisions, conventions, and knowledge as first-class, owned assets that survive any individual, any team change, and any model generation.
4. **Treat AI providers as a replaceable substrate** — extract each provider's full power without ever becoming hostage to any one of them; portability of *value*, not lowest-common-denominator features.
5. **Keep the human accountable and in command** — amplify engineering judgment; never silently substitute for it.
6. **Compound over the full engineering lifecycle, eventually** — from idea to deployment to knowledge update — while earning each stage's place rather than claiming the whole map up front.
7. **Be an open, ownable platform** — the value an organization builds on Foundry must belong to that organization, not be held hostage by Foundry.

---

## 3. Non-Goals

What Foundry is deliberately *not*, stated so we can refuse scope that violates the identity.

- **Not an IDE, editor, or autocomplete.** We do not compete for the cursor. Foundry orchestrates engineering work; it does not host the act of typing.
- **Not a chatbot or assistant.** Conversation may be *an* interface to Foundry, but Foundry is not a conversation. The product is the process and the record, not the dialogue.
- **Not a prompt manager or prompt-engineering tool.** Prompts are an implementation detail of skills. A platform whose central artifact is a prompt has bet on the wrong layer (§6.2).
- **Not an AI wrapper.** A wrapper's value is proportional to the model it wraps. Foundry's value must be *orthogonal* to the model — present even if every current model vanished.
- **Not an autonomous-agent product.** We are not selling "fire and forget; the AI ships to prod." Autonomy is a dial, not a destination, and accountability never leaves a human (§6.4).
- **Not a single-vendor model platform.** Foundry never becomes the go-to-market surface for one model provider. The day Foundry's identity depends on Provider X is the day it has failed goal 4.
- **Not a closed SaaS that holds your knowledge hostage.** An organization's accumulated knowledge and process must remain portable and owned by that organization (§7, Value V4).
- **Not a benchmark-chasing code generator.** We do not optimize for "lines of correct code per prompt" leaderboards at the expense of trust, auditability, or process quality.

---

## 4. Who Foundry Is For — And Who It Is Not

A product that is for everyone is for no one. Defining the non-audience is the more important and more neglected half.

### 4.1 The target audience

- **Engineering teams and organizations** who experience their *process* as an asset worth improving — who already write RFCs, keep ADRs, review code seriously, and feel the pain when that discipline lives only in people's heads.
- **Staff+ engineers and platform/infra teams** responsible for *how* an organization builds, not just *what* it ships — the people who would otherwise encode process in wikis, linters, and tribal knowledge.
- **Regulated, high-stakes, or long-lived codebases** where auditability and reproducibility are not luxuries: fintech, infrastructure, safety-critical, and any system expected to outlive the tenure of the people who built it.
- **Open-source maintainers and communities** who want process to be a shared, forkable, improvable artifact rather than the private habit of a core maintainer.

The unifying trait: **they believe process and knowledge compound, and they are willing to invest up front to make that happen.**

### 4.2 ⚠ Who Foundry is explicitly NOT for

This is a deliberate, load-bearing stance, and we should state it without apology:

- **The solo hacker optimizing for speed on throwaway code.** If the work won't outlive the week and no one will ever audit it, Foundry's process discipline is pure overhead. A chat box is the correct tool. We should *say so*.
- **Anyone who wants a faster chat box.** If the desire is "same conversation loop, but quicker," Foundry is the wrong shape and will feel heavier, not lighter. We will lose that user, and we should — trying to win them corrupts the product (§6.6).
- **Teams unwilling to invest in process.** Foundry taxes you up front and pays back in compounding. To a team that does not believe compounding is real, the tax looks like pure friction and the payback looks like a promise. We cannot and should not convert them by removing the discipline that *is* the product.
- **Believers in full autonomy.** Anyone whose goal is to remove humans from the accountability loop wants a different product and a different risk posture than the one we are willing to ship.

> **Why being this exclusionary is correct.** Every feature that makes Foundry attractive to the "faster chat box" crowd erodes the process discipline that makes it valuable to the target audience. The two audiences want opposite things. Trying to serve both produces a tool that is too heavy to be a good chat box and too shallow to be a good engineering platform. We pick a side, on purpose, forever.

---

## 5. Why Existing Tools Are Insufficient

Not a competitive teardown — a structural argument about what each category *cannot* do because of what it *is*.

| Category | What it is | What it structurally cannot do |
|---|---|---|
| **IDE-centric AI (Copilot, Cursor)** | Intelligence at the cursor, inside the act of editing | The unit is the keystroke/file. There is no first-class notion of a *process*, a *gate*, or an auditable *run*. Knowledge is the open buffer. It cannot make engineering work reproducible or accountable because that was never its altitude. |
| **Chat/agent assistants (Claude Code, generic agents)** | A conversation (possibly multi-step) that produces output | The artifact is the conversation; it does not compound, is not reproducible, and carries no durable record of *why* output was accepted. Quality rides on operator prompt skill. |
| **Autonomous agents (Devin-class)** | "Give it a task; it ships" | Trades away the human accountability loop. Removes the human from the place engineering organizations legally and culturally require one. Optimizes autonomy, the thing we deliberately do *not* optimize (§6.4). |
| **Prompt-engineering / prompt-management tools** | Versioned, tested prompts | Bets the entire value on the prompt — the most model-coupled, fastest-commoditizing artifact in the stack. When the model changes, the asset depreciates (§6.2). |
| **CI / quality platforms** | Deterministic verification of artifacts | Verifies the *output* of engineering but does not *orchestrate* it, does not assemble the knowledge that informs it, and has no model of intent → implementation. It checks the diff; it does not produce or reason about it. |

The pattern: each category owns one slice and treats the others as out of scope. **No existing category treats the full engineering process — intent, knowledge, generation, verification, accountability, and the record of all of it — as the product.** That is the space Foundry occupies, and it is empty not because it is small but because it is hard and unglamorous.

---

## 6. Core Principles

These are the project's invariants. Each is stated so it can be used to *reject* a proposal, not merely to feel good. If a principle cannot kill a feature, it is decoration.

### 6.1 Trust is the product

The thing Foundry actually manufactures is **trust** — the property that makes machine-produced engineering work safe to merge, deploy, and depend on. Code generation is the commodity input; trust is the output and the moat. Concretely, trust is composed of **provenance** (we can show what informed every output), **verification** (we checked it deterministically wherever possible), **accountability** (a human owns the decision), and **reproducibility** (we can show how we got here).

> **As a rejection test:** Any feature that produces output faster but reduces our ability to explain, verify, attribute, or reproduce it is moving in the wrong direction, regardless of how impressive the output is.

### 6.2 Software Engineering > Prompt Engineering — made falsifiable

The founding slogan is true but, as a slogan, empty. It earns its place only when operationalized into claims that could be wrong:

1. **Control flow is owned by deterministic process, never by a model.** A model fills in a step; it never decides which step runs next, whether a gate passed, or whether work is done.
2. **Every model output is an untrusted input until verified.** The model proposes; the process disposes. Verification is deterministic-first.
3. **The process is the durable artifact; the prompt is disposable plumbing.** Prompts live *inside* skills and may be rewritten freely when models change. The skill's *contract* — what it requires, produces, and guarantees — is what we version and protect.

> **As a rejection test:** If a proposal puts a model in charge of control flow, treats model output as trusted-by-default, or elevates a prompt to a first-class user-facing artifact, it violates this principle.

### 6.3 Knowledge is durable capital; the model is a replaceable substrate

The assets that would still be valuable if every current model disappeared tomorrow — an organization's decisions, conventions, rationale, and the record of how its systems came to be — are the ones Foundry must treat as first-class and *owned*. The model is rented compute behind a swappable boundary.

This has a sharp corollary the founding brief gets right and most of the industry gets wrong: **"replaceable" does not mean "lowest common denominator."** Refusing to use a provider's caching, reasoning, or tool capabilities in the name of portability would be self-defeating — it would make Foundry worse to avoid lock-in it could avoid by other means. Foundry exploits each provider's full power *and* remains free to leave. The portability we guarantee is portability of **value** (your knowledge, your process, your history), not portability of every feature.

> **As a rejection test:** Any decision that makes an organization's accumulated knowledge or process *depend on* a specific model or provider — such that switching providers would destroy it — violates this principle.

### 6.4 The human is the source of intent and the holder of accountability

Foundry amplifies engineering judgment; it does not replace it. The default posture is **approval, not autonomy**: Foundry proposes, a human disposes, and outward or irreversible actions require explicit human authorization. Autonomy is a *dial the user may turn up* for well-understood, low-blast-radius work — never a destination the product pushes toward.

This is a values choice, not a capability limit. Even if a model were good enough to ship to production unattended, Foundry would not *default* to letting it, because accountability for engineering decisions belongs to a person and an organization, and a tool that quietly relieves humans of that accountability is selling a liability disguised as a feature.

> **As a rejection test:** Any feature whose appeal is "the human no longer has to be involved / accountable" violates this principle. Reducing *toil* is good; removing *accountability* is not.

### 6.5 Reproducibility over reproduction — honest determinism

Models are not deterministic and will not become so; promising bit-identical model output would be a lie that erodes trust (and trust is the product, §6.1). Foundry therefore commits to **process determinism**, not output determinism: the *structure* of a run — which stages execute, under which gates, against which knowledge, producing which auditable record — is reproducible and replayable, even though the model's text is not. We promise the process around the model is rigorous, observable, and replayable, and we communicate that distinction relentlessly and honestly.

> **As a rejection test:** Any claim or feature that implies Foundry makes *model output* deterministic is dishonest and must be reframed or rejected.

### 6.6 Ceremony must be earned by value — the simple path stays simple

The characteristic death of a process platform is ceremony that outweighs the value it produces: the tool becomes heavier than the problem, and people route around it. Foundry's process discipline must always be *proportional to the stakes of the work*. The trivial task must have a trivial path; the load-bearing, audited, irreversible task earns the full apparatus. Discipline is a service to the user, never a tax collected for its own sake.

> **As a rejection test:** Any feature that adds mandatory ceremony to *all* work to serve the needs of *some* work violates this principle. Make the apparatus opt-in by stakes, not mandatory by default.

### 6.7 Depth before breadth — earn each stage of the lifecycle

The vision spans the entire lifecycle (§7 of the architecture doc). The strategy does not. Foundry must do *one* complete loop — intent → implementation → verification → knowledge update — genuinely well before widening to RFCs, security, release, and deployment. Each new lifecycle stage must independently justify its existence against this RFC before it is built. A platform that claims the whole map before it owns a single territory dies of its own ambition.

> **As a rejection test:** "We should add stage X because the vision includes it" is not a justification. "Stage X compounds value for users who already get value from the existing loop" is.

### 6.8 Boring where it counts

Foundry acts on people's real repositories and performs irreversible actions. The core of the system — the part that decides, verifies, and records — must be conservative, legible, and recoverable. Cleverness belongs at the replaceable edges (skills, providers), never in the part the user must trust with their codebase.

---

## 7. Values That Must Never Be Compromised

Principles guide decisions; values are the lines we do not cross even under pressure. Compromising any of these is a betrayal of the product's identity, not a trade-off.

- **V1 — Accountability stays with a human.** We will not ship a default that removes a person from ownership of consequential engineering decisions.
- **V2 — The user's source of truth is never silently mutated.** Foundry proposes changes to code and to knowledge as reviewable artifacts. It never rewrites an organization's authoritative decisions or working state behind their back.
- **V3 — Every consequential action is auditable.** If Foundry did something that mattered, there is an immutable record of what, why, on what evidence, and by whose approval. Auditability is not an enterprise upsell; it is the product.
- **V4 — The organization owns its knowledge and process, and they are portable.** What you build *on* Foundry belongs to *you* and can leave *with* you. We never hold an organization's engineering capital hostage to keep them as a customer.
- **V5 — We are honest about what AI can and cannot guarantee.** No determinism we cannot deliver, no trust we have not verified, no autonomy we have not made safe. Overselling AI is the fastest way to destroy the trust that is our product.
- **V6 — No vendor capture.** Foundry never becomes structurally dependent on, or the marketing surface for, a single model provider.

---

## 8. Design Philosophy

How the principles translate into a stance on design — still at the altitude of philosophy, not mechanism. (Mechanism lives in `ARCHITECTURE.md` and later RFCs, which must conform to what follows.)

### 8.1 The loop is the product, not the pipeline

> **⚠ Challenge to the founding vision.** The brief draws the lifecycle as a linear sequence: Idea → RFC → … → Release → Deployment → Knowledge Update. Drawn that way, it is wrong in a way that matters.

Real engineering is not a pipeline that runs once and terminates. It is a **cycle**, and the single most important arrow is the one the linear drawing hides: **Knowledge Update closing back to the top.** A platform that executes work and forgets is a fancier chat box. A platform that executes work *and is measurably better at the next task because of what the last task taught it* is a different category of thing. The compounding is the entire thesis (§1.2, goal 3). Every design decision should be evaluated against whether it strengthens or weakens that closing loop.

Furthermore, real engineering is **interrupt-driven and non-linear** — work is paused, resumed, abandoned, and revisited; stages are skipped or repeated. The design must treat the lifecycle as a graph of resumable, replayable states, not a conveyor belt. (The architecture's run-ledger and resumable-state design is the correct embodiment of this; it should be understood as *serving this principle*, not as an implementation detail that happens to exist.)

### 8.2 Separate the durable from the disposable, ruthlessly

The central design tension is between things that should last a decade and things that will be obsolete in a quarter. The philosophy is to keep them on *opposite sides of a hard boundary*:

- **Durable** (protect, version, own): the engineering process, the organization's knowledge and decisions, the verification rules, the auditable record of runs.
- **Disposable** (expect to churn, keep replaceable): prompts, model choices, provider-specific features, the particular model generation in use.

Every artifact in Foundry should be classifiable as one or the other, and the disposable must never become load-bearing for the durable. When a model generation changes, the durable assets should be untouched; only the disposable layer adapts.

### 8.3 Deterministic-first, model-last

Wherever a question can be answered by deterministic means — a compiler, a test, a type-checker, a structural rule, an exact search over a knowledge graph — it must be, *before* a model is consulted. The model is the most expensive, least explainable, and least reproducible tool available; it is the tool of last resort for the irreducibly subjective, not the default tool for everything. This is §6.2 applied to design: the more of Foundry's behavior that is deterministic, the more of it is trustworthy, cheap, and reproducible.

### 8.4 Design for graceful, diagnosable failure — not for an oracle

There is no technique that makes AI-assisted engineering reliably correct. The platform will sometimes assemble the wrong knowledge, generate the wrong code, or accept something it should have rejected. The philosophy is not "be perfect"; it is to ensure that **every failure is contained, diagnosable, and a source of learning**: contained by verification gates and human approval, diagnosable by provenance and the run record, and a source of learning by the feedback loop (§8.1). We design assuming the model is fallible, because it is.

### 8.5 Honesty as an architectural property, not a tone of voice

Because trust is the product (§6.1) and overselling is its fastest solvent (§V5), honesty is not merely a communication style — it is a design constraint. The system must *structurally* tell the truth: surface what it actually knows, show the provenance of its claims, distinguish what it verified from what it inferred, and refuse to imply guarantees it cannot keep. A design that makes the system look more certain or more autonomous than it is has a defect, not a polish problem.

---

## 9. Success Criteria

### 9.1 What success looks like in five years

Vanity metrics (stars, installs) are explicitly *not* the definition. Success is defined by whether the bet in §1.2 paid off:

1. **Survival across model generations.** Foundry runs and the assets they produced survive multiple model-generation transitions with the *durable* layer untouched — organizations swap the substrate and keep their process and knowledge intact. This is the direct test of goals 3 and 4.
2. **Demonstrable compounding.** Teams using Foundry can *show* that their engineering knowledge accumulated and that later work measurably benefited from earlier work — the closing loop (§8.1) is real, not aspirational.
3. **"Foundry-verified" means something.** The phrase carries weight — it implies a real, auditable standard of how a change was produced, checked, and approved — to people who are not Foundry users.
4. **An ecosystem the core did not build.** A thriving body of community-authored process, knowledge sources, verification, and ecosystem support exists that the core team never wrote — evidence that the platform genuinely opened a space rather than being a closed product (goal 7).
5. **Trust under audit.** An organization in a regulated context can answer "how was this built and why should we trust it?" using Foundry's record alone, and pass an audit on that basis.

### 9.2 ⚠ What failure looks like (so we can detect it early)

A vision document that only describes success is unfalsifiable. Foundry has *failed at its identity* — even if it is commercially alive — if any of the following becomes true:

- It is, in practice, used as a faster chat box, and its process/knowledge features are vestigial.
- Its value is materially tied to one model provider, such that the provider's moves dictate Foundry's.
- An organization's accumulated knowledge cannot leave Foundry intact (V4 violated).
- "Foundry-verified" means nothing because verification was watered down to reduce friction (§6.6 over-applied, §6.1 abandoned).
- The default posture drifted toward autonomy and humans are routinely out of the accountability loop (V1 violated).

We should monitor for these failure signatures the way we would monitor production SLOs.

---

## 10. Trade-offs

The conscious costs of this identity. Naming them is how we avoid pretending the chosen path is free.

| We chose | Over | The cost we knowingly accept |
|---|---|---|
| Process & knowledge as the product | Best-in-class code generation | We will sometimes generate worse code than a tool that optimizes only for generation. We bet trust + compounding beats raw generation over a decade. |
| A narrow, demanding target audience | Mass appeal | We deliberately lose the large "faster chat box" market. Slower adoption; stronger identity. |
| Approval-by-default (§6.4) | Autonomy-by-default | We will feel slower and heavier than autonomous agents on tasks where their risk posture is acceptable. We accept looking less magical to preserve accountability. |
| Honest, partial determinism (§6.5) | A clean "deterministic AI" marketing story | We give up a seductive pitch in exchange for not lying. Harder to market; impossible to be caught overselling. |
| Provider-agnostic value (§6.3) | Deep single-vendor optimization as identity | More work to support breadth and negotiate capabilities; we are never the "best Provider-X experience," on purpose. |
| Depth before breadth (§6.7) | Claiming the full lifecycle now | The vision looks under-delivered early; impatient observers will call it small. We accept that to avoid dying of ambition. |
| Ceremony proportional to stakes (§6.6) | A single uniform rigorous path | More complexity in meeting users where they are; we accept it rather than tax trivial work into abandonment. |

---

## 11. Open Questions

Genuine unknowns this RFC does not resolve. Each is a candidate for a future RFC or for explicit deferral.

1. **The compounding proof.** Goal 3 and success criterion §9.1.2 assert that knowledge compounds and later work benefits. *How do we measure that, concretely, in a way a skeptical user would accept?* Without a measure, "compounding" risks becoming the same unfalsifiable promise we criticized in §1.
2. **The adoption-curve contradiction.** Foundry taxes up front and pays back over time (§4.2, §6.6), but its narrow audience (§4) and approval-by-default posture (§6.4) both *slow* the early adoption that would generate the knowledge that demonstrates the payback. *How does Foundry deliver enough day-one value to a single user to survive long enough to reach the compounding regime?* This is the most dangerous tension in the entire vision and it is currently unresolved.
3. **Where does intent come from, and how is it captured?** The human is the source of intent (§6.4), but the lifecycle starts at "Idea." *What is the philosophy of how raw human intent enters the system without becoming just another chat box at the front door?* This may deserve its own RFC.
4. **The boundary of "knowledge."** §6.3 and §8.2 lean heavily on "knowledge" as durable capital. *What is, and is not, knowledge — and who decides?* Code structure is derivable; a decision's rationale is authored. The line between them is doing a lot of philosophical work and is not yet rigorously defined here.
5. **Autonomy's safe envelope.** §6.4 says autonomy is "a dial, not a destination." *What principles govern how far that dial may safely turn, and for what classes of work?* We have asserted the default; we have not defined the envelope.
6. **The open-source / sustainability model.** Goal 7 and V4 demand an open, ownable platform. *What governance and sustainability model keeps Foundry open and ownable without collapsing — and without the V6 vendor-capture pressure creeping in through funding?* This is a product-survival question, not just a licensing detail.
7. **The name.** "Foundry" is provisional and collides with existing tools in adjacent spaces. *Does the name survive, and does it matter?* Deferred, but logged.

---

## 12. Future RFC Dependencies

RFC-0001 is the root. The following are anticipated children that *depend on* this RFC and must conform to it. (Numbers are indicative, not committed.)

- **RFC-0002 — The Engineering Lifecycle Model.** Defines the cycle (not pipeline) of stages, the closing knowledge loop, and the criteria a stage must meet to earn inclusion (operationalizes §6.7, §8.1).
- **RFC-0003 — Knowledge Philosophy: Durable vs. Derived.** Rigorously defines what knowledge is, the authored/derived distinction, and ownership/portability guarantees (resolves Open Question 4; serves §6.3, V4).
- **RFC-0004 — The Trust & Verification Model.** Defines what "Foundry-verified" means, the deterministic-first stance, and the trust composition of provenance/verification/accountability/reproducibility (operationalizes §6.1, §6.5, §8.3).
- **RFC-0005 — The Human-in-Command Model.** Defines the approval-vs-autonomy posture, the blast-radius classification of actions, and the safe autonomy envelope (resolves Open Question 5; serves §6.4, V1).
- **RFC-0006 — Provider Neutrality & Capability Philosophy.** Defines portability-of-value, capability negotiation as philosophy (not LCD), and the anti-vendor-capture stance (serves §6.3, V6).
- **RFC-0007 — Intent Capture.** Defines how human intent enters Foundry without regressing to a chat box (resolves Open Question 3).
- **RFC-0008 — Adoption & Value-on-Day-One.** Confronts the §11 Open Question 2 contradiction head-on: how Foundry delivers standalone value before the compounding regime.
- **RFC-0009 — Openness, Governance & Sustainability.** Defines how Foundry stays open and ownable without vendor capture (resolves Open Question 6; serves goal 7, V4, V6).

The existing `docs/ARCHITECTURE.md` is **downstream** of this RFC and of RFCs 0002–0006. Where it currently cites "the brief," it should, post-ratification, cite the relevant principle or RFC by number. Where it conflicts with this RFC, it must be amended.

---

## 13. How Future Architectural Decisions Must Be Evaluated

This is the most durable thing this RFC produces: a decision rubric. **Every future RFC, ADR, and significant design decision must answer these questions in writing.** A proposal that cannot answer them is not ready.

1. **Durable or disposable?** Does this increase the *durable* value (process, knowledge, trust, record) or does it deepen our dependence on the *disposable* layer (a model, a provider, a prompt)? (§8.2)
2. **Provenance & audit.** Does it preserve our ability to explain, attribute, verify, and reproduce what the system did? Or does it create output we cannot account for? (§6.1, V3)
3. **Control flow.** Does deterministic process remain in command, or does it hand control flow to a model? (§6.2)
4. **Accountability.** Does a human remain the owner of consequential decisions, or does this quietly remove them from the loop? (§6.4, V1)
5. **The simple path.** Does this add ceremony to *all* work to serve *some* work? Does the trivial task stay trivial? (§6.6)
6. **Vendor capture.** Does this make us dependent on, or a marketing surface for, a single provider? Does it make the user's value non-portable? (§6.3, V4, V6)
7. **Honesty.** Does it make the system imply a guarantee — of determinism, correctness, or autonomy — that we cannot actually keep? (§6.5, V5)
8. **The loop.** Does it strengthen or weaken the closing Knowledge-Update arrow — the compounding that is the whole thesis? (§8.1)
9. **Reversibility of the decision itself.** How expensive is it to undo this if we are wrong? Prefer decisions that are cheap to reverse; demand much stronger justification for ones that are not. (§6.8)

A decision that scores well on speed or capability but poorly on this rubric is, by definition, off-mission — no matter how attractive the demo.

---

## 14. How Contributors Should Think About the Project

For everyone who builds Foundry or builds on it:

- **You are building durable engineering capital, not AI features.** The most valuable thing you can contribute is something still useful in ten years and across five model generations. If your contribution's value evaporates when the model changes, you are working on the disposable layer — which is fine, but know which layer you are on, and never let it become load-bearing for the durable one.
- **The edges are where cleverness belongs; the core is where boredom belongs.** Innovate freely in skills, providers, knowledge sources, and verification. In the part of the system the user trusts with their repository, prefer conservative, legible, recoverable design. (§6.8)
- **This document is meant to be argued with.** Disagreement is contribution. The right way to challenge a principle is to open an RFC or ADR that cites the section number and makes the counter-argument — not to route around it in code. A principle that survives a serious challenge is stronger; one that cannot survive should be amended.
- **When in doubt, run the rubric (§13).** It is not bureaucracy; it is the compressed judgment of this entire document. If a choice is unclear, the rubric usually makes the mission-aligned option obvious.
- **Optimize for conceptual clarity and longevity over implementation speed.** A fast feature that muddies the model or couples us to the disposable layer is a long-term loss disguised as a short-term win. We are building something meant to last a decade; act like it.

---

_End of RFC-0001. Status: Proposed. Challenge any section by opening an RFC or ADR that references its number. This document becomes the foundation every future architectural decision answers to — which means it is only as good as the scrutiny it survives. Scrutinize it._
