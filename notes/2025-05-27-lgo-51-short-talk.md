# Beyond Benchmarks: Human x LLM for (Go) Code (intermediate report)

> 2025-05-27, [Leipzig Gophers](https://golangleipzig.space) #51, Martin Czygan

## Motivation

* not really convinced, but tried to make a more systematic effort to use
  coding tools since 01/2025 - and to document the process:
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
| 🇺🇸 OpenAI                      | GPT-4.1                             | no   | June 2024        | 1M                       | Great overall performance                                         | -                          | default                                 |
| 🇺🇸 OpenAI                      | GPT-4.1 Mini                        | no   | June 2024        | 1M                       | Fast overall performance                                          | -                          | default                                  |
| 🇺🇸 OpenAI                      | o1                                  | no   | Oct 2023         | 128k                     | Good overall performance, reasoning                               | no streaming               | default                                 |
| 🇺🇸 OpenAI                      | o1-mini                             | no   | Oct 2023         | 128k                     | Fast overall performance, reasoning                               | no streaming               | default                                 |
| 🇺🇸 OpenAI                      | GPT-4o                              | no   | Oct 2023         | 128k                     | Good overall performance, vision                                  | -                          | default                                 |
| 🇺🇸 OpenAI                      | GPT-4o Mini                         | no   | Oct 2023         | 128k                     | Fast overall performance, vision                                  | -                          | default                                 |


* prompt engineering is really model training (or "[in-context learning](https://arxiv.org/pdf/2301.00234)"), it
  just does not necessary feel that way

## TL;DR

* so far, both: HITS and MISSES
* feels like early Stack Overflow, helpful to fill in missing pieces; augmentation to docs
* throwaway code, prototypes, able to skip stuff I am not interested in

## trcli

Wanted to have a CLI tool for accessing TRELLO board and printing out.

* 185 lines of code, [claude/chat](https://claude.ai/chat/79da6368-24b9-4d17-bbf7-df57b0219b3b)

## cli palm tree

* I love palm trees and the cli
* can I get a palm tree into my terminal?

Short answer: NO!

