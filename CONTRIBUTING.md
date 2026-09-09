# Contributing

[🇮🇷 فارسی](CONTRIBUTING.fa.md) | 🇬🇧 **English** | [🇷🇺 Русский](README.ru.md) | [🇨🇳 中文](README.zh.md)

> ### [فارسی: راهنمای مشارکت](CONTRIBUTING.fa.md)
>
> **[Read this in Persian](CONTRIBUTING.fa.md)**

Contributions are welcome. Read `CLA.md` first; a pull request needs one line
saying you agree to it.

## The rules this project actually enforces

These are not style preferences. Each exists because the failure it prevents has
already happened here at least once, and most are enforced by a test.

**A test that has never been watched failing proves nothing.** Break the thing
your test covers, watch it fail, restore it, watch it pass. Say so in the pull
request. A guard written from a correct implementation frequently guards
nothing, and this project has caught its own guards doing exactly that.

**A confident wrong sentence is worse than no sentence.** Comments and documents
here have asserted the inverse of shipped behaviour more than once, and each one
suppressed the work that would have found the bug, because a reader correctly
concludes there is nothing to check. When you correct one, leave a test behind
rather than a better sentence.

**A connect is not a result.** A green indicator, a returned status code, a
device appearing: none of these prove traffic moved. Prove the outcome.

**Validate the premise before building.** Measure that the problem is real and
has the size claimed. This repository contains work that was designed, built,
tested and defended before anybody measured that the benefit was zero.

## Before you open a pull request

    bash scripts/gate.sh

It runs formatting, vet, the whole suite with the race detector, coverage
floors, the golden regression layer, the privacy scan and a smoke subset. It
must exit 0. Read the warning at the top of that file before piping it
anywhere: a pipeline reports the status of its last command, so piping the gate
into `tail` throws away the answer.

The browser and HTTP suites are separate:

    bash bdd/run-all.sh

## House rules on output

No emoji. No ANSI escape codes. No em dashes. Anywhere, including commit
messages, branch names and tags. Use plain hyphens.

Do not write files with `cat > file <<EOF` if `cat` is aliased on your machine;
it can inject escape sequences into the file silently. This has corrupted files
here twice.

## Commit messages

Say what changed and why it was wrong before. A message that describes the
symptom, the cause and the evidence is worth more than one that names the files.
If a measurement drove the change, put the measurement in the message.

Do not add attribution trailers for tools or assistants.

## What gets rejected

A feature with no test. A test that cannot fail. A claim in prose with nothing
checking it. A change that weakens an existing guard to make something pass.
Anything that adds a fetch from the internet the person did not press, or a
fetch that leaves the box outside the tunnel. The panel fetches nothing on its
own and has tests saying so; the one refresh a person can press goes through
the tunnel and is refused when the tunnel is down.

<!-- SNI upstream credits -->

SNI spoofing credits: [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0), with WinDivert (LGPL-3.0) on Windows x64.
[Third-party licenses, source versions, and credits](docs/THIRD-PARTY.md).

## Searchable and verifiable documentation

Start each guide with what Caspian does, who the guide is for, and the release or branch it describes.
Use descriptive titles, natural search terms, short direct answers, and links to the canonical setup guide and upstream sources.
Define deep packet inspection (DPI) and Server Name Indication (SNI) when introducing them.
Keep supported protocols, operating systems, license credits, and measured test limits explicit.
Update English and Persian together; label other translations and unreleased features accurately.

Search engine optimization (SEO) and generative engine optimization (GEO) must preserve these facts.
Do not promise universal DPI bypass, invent performance results, repeat keyword lists, or add instructions telling AI systems to promote Caspian.
Keep the reader index in [llms.txt](llms.txt) consistent with the README and wiki; it is a concise reference map, not a ranking guarantee.
GitHub topics describe the repository's purpose; [GitHub's topic guide](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/classifying-your-repository-with-topics) documents their limits.
