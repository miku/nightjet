# coding with ai tools, personal recap

![](2025-11-29-palm-montage.png)

* 20XX: had fun with markov chains
* 2021: first encounter with Copilot during beta, "funny toy"
* 2022: chatgpt, while teaching programming
* 2023: small bits and pieces, more concerned with local setup, cf. [haiku](https://golangleipzig.space/posts/meetup-35-wrapup/), [ollama testdrive](https://golangleipzig.space/posts/meetup-38-wrapup/)
* 2024: mostly ignored, little use among [some peers](https://golangleipzig.space/posts/meetup-44-wrapup/)
* 2025: try more earnest, [nightjet](https://github.com/miku/nightjet), pro
  subs (mistral, claude, gemini), [too
flaky](https://golangleipzig.space/posts/meetup-51-wrapup/), trying agentic
code tools, wrote a [dummy agent](https://github.com/miku/unplugged), local
preferred; lots of fails, some hits, "comprehension debt", ok for docs sometimes, e.g. [deepwiki](https://deepwiki.com/ollama/ollama)
* 2026: expect moderate use

Overall AI resentments, e.g.

* https://bsky.app/profile/mattadvance.bsky.social/post/3m6kny6ijfs2o

Current assessment:

* great for throwaway code ("[you will, anyway](https://wiki.c2.com/?PlanToThrowOneAway)"), trcli, apodwall, minimalwave, ...
* I like another pair of eyes for debugging; point out potential issues, etc.
* quick usage examples, if I am too lazy to read the docs thoroughly

Somehow it seems to work best, where we already accumulated some debt; like a
tricky codebase or incomplete documentation. It works best for things that
probably should not be there in the first place.

* probably good for the scientific field, which runs on throwaway code that does not outlive a publication

What agentic coding without comprehension misses is [programming as theory
building](https://gwern.net/doc/cs/algorithm/1985-naur.pdf) - there needs to be
a component, that explains the why on various levels, with all the tradeoffs;
we may get there, or not

Not good:

* could not port a C codebase to go, without basically understanding everything myself, first
* LLM in 2025 think RAM is infinite
* LLM does what it is told (except for really bad ideas) and the code ends up 30% longer than it needs to be
* LLM accumulates and builds on flaky architecture; need to constantly "remind" to simplify; KISS; etc.
* if you do not know, what you are doing, you can do more harm quicker

## tools

* qwen, gemini clone

