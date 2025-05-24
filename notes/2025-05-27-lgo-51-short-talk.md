# Beyond Benchmarks: Human x LLM for (Go) Code (intermediate report)

> 2025-05-27, [Leipzig Gophers](https://golangleipzig.space) #51, Martin Czygan

## Motivation

* not really convinced, but tried to make a more systematic effort to use
  coding tools since BOY 2025 - and to try to document the process:
[nightjet](https://github.com/miku/nightjet)

Testing various open models.

| Organization                     | Model                               | Open | Knowledge cutoff | Context window in tokens | Advantages                                                        | Limitations                | Recommended settings                    |
|----------------------------------|-------------------------------------|------|------------------|--------------------------|-------------------------------------------------------------------|----------------------------|-----------------------------------------|
| 🇺🇸 Meta                        | Llama 3.1 8B Instruct               | yes  | Dec 2023         | 128k                     | Fastest overall performance                                       | -                          | default                                 |
| 🇺🇸 Google                      | Gemma 3 27B Instruct                | yes  | Mar 2024         | 128k                     | Vision, great overall performance                                 | -                          | default                                 |
| 🇨🇳 OpenGVLab                   | InternVL2.5 8B MPO                  | yes  | Sep 2021         | 32k                      | Vision, lightweight and fast                                      | -                          | default                                 |
| 🇨🇳 Alibaba Cloud               | Qwen 3 235B A22B                    | yes  | Sep 2024         | 32k                      | Great overall performance, multilingual, global affairs, logic    | -                          | default                                 |
| 🇨🇳 Alibaba Cloud               | Qwen 3 32B                          | yes  | Sep 2024         | 32k                      | Good overall performance, multilingual, global affairs, logic     | -                          | default                                 |
| 🇨🇳 Alibaba Cloud               | Qwen QwQ 32B                        | yes  | Sep 2024         | 131k                     | Good overall performance, reasoning and problem-solving           | Political bias             | default, temp=0.6, top_p=0.95           |
| 🇨🇳 DeepSeek                    | DeepSeek R1                         | yes  | Dec 2023         | 32k                      | Great overall performance, reasoning and problem-solving          | Censorship, political bias | default                                 |
| 🇨🇳 DeepSeek                    | DeepSeek R1 Distill Llama 70B       | yes  | Dec 2023         | 32k                      | Good overall performance, faster than R1                          | Censorship, political bias | default, temp=0.7, top_p=0.8            |
| 🇺🇸 Meta                        | Llama 3.3 70B Instruct              | yes  | Dec 2023         | 128k                     | Good overall performance, reasoning and creative writing          | -                          | default, temp=0.7, top_p=0.8            |
| 🇩🇪 VAGOsolutions x Meta        | Llama 3.1 SauerkrautLM 70B Instruct | yes  | Dec 2023         | 128k                     | German language skills                                            | -                          | default                                 |
| 🇫🇷 Mistral                     | Mistral Large Instruct              | yes  | Jul 2024         | 128k                     | Good overall performance, coding and multilingual reasoning       | -                          | default                                 |
| 🇫🇷 Mistral                     | Codestral 22B                       | yes  | Late 2021        | 32k                      | Coding tasks                                                      | -                          | temp=0.2, top_p=0.1, temp=0.6, top_p=0.7|
| 🇺🇸 intfloat x Mistral          | E5 Mistral 7B Instruct              | yes  | -                | 4096                     | Embeddings                                                        | API Only                   | -                                       |
| 🇨🇳 Alibaba Cloud               | Qwen 2.5 72B Instruct               | yes  | Sep 2024         | 128k                     | Good overall performance, multilingual, global affairs, logic     | -                          | default, temp=0.2, top_p=0.1            |
| 🇨🇳 Alibaba Cloud               | Qwen 2.5 VL 72B Instruct            | yes  | Sep 2024         | 90k                      | Vision, multilingual                                              | -                          | default                                 |
| 🇨🇳 Alibaba Cloud               | Qwen 2.5 Coder 32B Instruct         | yes  | Sep 2024         | 128k                     | Coding tasks                                                      | -                          | default, temp=0.2, top_p=0.1            |

* prompt engineering is really model training (or "[in-context learning](https://arxiv.org/pdf/2301.00234)"), it
  just does not necessary feel that way

> It has been a significant trend to explore ICL to evaluate and extrapolate
> the ability of LLMs. [A Survey on In-context
> Learning](https://arxiv.org/pdf/2301.00234) (2024)

## TL;DR

* so far, both: HITS and MISSES
* feels like early Stack Overflow (SO), helpful to fill in missing pieces; augmentation to docs
* throwaway code, prototypes, able to skip stuff I am not interested in
* just like SO, it's all fine as long as you understand every line (up to some granularity)

## trcli

Wanted to have a CLI tool for accessing TRELLO board and printing out.

* 185 lines of code, [claude/chat](https://claude.ai/chat/79da6368-24b9-4d17-bbf7-df57b0219b3b)

## cli palm tree

* I love palm trees and the cli
* can I get a palm tree into my terminal?

Short answer: NO!

## metha refactoring

* command line harvester for XML data; wanted zstd support
* codebase existing and familiar
* overall useful, somewhat easier to review code; need backwards compatibilty and one function needed more parameters
* 181 SLOC of migration script; to port existing cache from gzip to zstd; one-time, low risk operation (it is only a cache; saved bandwidth); required additional review for special case

The LLM suggested compression level, but I really did not care about that.

```go
// Helper to add the appropriate extension based on compression type
func compressedFilename(base string, compressionType CompressionType) string {
        switch compressionType {
        case CompZstd:
                return base + ".zst"
        default:
                return base + ".gz"
        }
}
```

Changed less than 100 SLOC.


```
$ git diff --stat 0a5555b4 -- harvest.go client.go cmd/metha-sync/main.go
 client.go  | 27 ++++++++++++++++++++++++---
 harvest.go | 40 ++++++++++++++++++++++++++++++++++++----
 2 files changed, 60 insertions(+), 7 deletions(-)
```

## replace.c port to Go

