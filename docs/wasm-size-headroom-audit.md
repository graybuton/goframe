# WASM Size Headroom Audit

## Summary

This document began with a toolchain-sensitive dashboard size investigation.
The historical Go `1.24.4` and Go `1.22.12` results remain below for context.

The current supported release-size source of truth is Go `1.26.5` plus TinyGo
`0.41.1` on Linux amd64. The 2026-07-30 migration reproduced the frozen base
twice with isolated workspaces and caches. Both runs produced identical raw,
gzip, Brotli, and Zstandard outputs for all eleven applications. Three measured
cells exceeded their previous limits, so only those cells were aligned to the
new baseline.

The controlled-select correctness correction later added a measured shared raw
runtime cost of `514 B` to `526 B`. The 2026-07-31 closeout reproduced the base
and behavior-correct head twice, evaluated four bounded compactness candidates,
and aligned only four raw ceilings by one KiB. No compressed ceiling, ratio,
runtime behavior, application code, or package behavior changed in that
closeout.

The descendant-component review follow-up adds `387 B` to `416 B` of raw WASM
over that closeout. Every raw, Brotli, and Zstandard cell remains within its
existing ceiling. The only old-ceiling failure is `resource` gzip at `68636 B`,
so that one ceiling is aligned from `68608 B` to `69632 B`.

The repeated-Mount nested-target guard adds `216 B` to `235 B` of raw WASM over
the reviewed repeated-Mount head. Exactly three old-ceiling cells fail:
`router` raw by `40 B`, `router-dashboard` raw by `134 B`, and `resource`
Zstandard by `74 B`. Those three ceilings increase by `1024 B`; every other
absolute ceiling and all ratio limits remain unchanged.

## Source of Truth

- Workflow: `.github/workflows/ci-wasm-size.yml`
- Budget script: `scripts/size-budget.sh`
- CI Go version: `1.26.5`
- CI TinyGo version: `0.41.1`

The workflow installs `goxc`, generates and packages every listed example with
TinyGo, then repeats packaging with `--asset-hash --preload
--compress=gzip,br` before running `scripts/size-budget.sh`.

The budget script does not build examples. It checks existing package artifacts
under `examples/<app>/.goframe/package/standalone`. For each app, it selects
the first path matching:

1. `assets/bundle*.wasm`
2. `main.wasm`

If no match exists, it reports the default missing path
`assets/bundle.wasm`.

### Budgets

| app | raw | br | gzip | zstd |
| --- | ---: | ---: | ---: | ---: |
| counter | 97280 B | 40960 B | 56320 B | 49152 B |
| components | 107520 B | 43008 B | 56320 B | 49152 B |
| todo | 122880 B | 40960 B | 56320 B | 49152 B |
| dashboard | 171008 B | 53248 B | 71680 B | 61440 B |
| context | 117760 B | 36864 B | 46080 B | 40960 B |
| virtualized | 124928 B | 40960 B | 49152 B | 44032 B |
| multipackage | 110592 B | 43008 B | 56320 B | 49152 B |
| cmdapp | 110592 B | 43008 B | 56320 B | 49152 B |
| router | 118784 B | 45056 B | 58368 B | 51200 B |
| router-dashboard | 235520 B | 77824 B | 94208 B | 82944 B |
| resource | 157696 B | 58368 B | 69632 B | 62464 B |

## Supported Toolchain Baseline Migration - 2026-07-30

The frozen base is `ed88c68328ca0c969c816c2191732751f9af10dd`.
Measurements use Go `1.26.5`, TinyGo `0.41.1` with LLVM `20.1.1`, Linux
amd64, gzip `1.13`, Brotli `1.1.0`, and Zstandard `1.5.7`. Each isolated run
installed its own `goxc`, generated and packaged all eleven applications, and
then repeated packaging with `--asset-hash --preload --compress=gzip,br`.

Two frozen-base runs with separate workspaces, Go caches, module caches, TinyGo
caches, and output directories were byte-identical. Repeating the package
sequence on the final workflow and budget state produced the same artifacts.

| app | raw | gzip | br | zstd | raw WASM SHA-256 (base and final) |
| --- | ---: | ---: | ---: | ---: | --- |
| counter | 84536 B | 34007 B | 28303 B | 30571 B | `c503b7d0d8c11a3a11123626f972bb494b6acad1835c014c892d6427931d1d29` |
| components | 90176 B | 35788 B | 29714 B | 31968 B | `c6e2a7076a3a638f15d73ec8bd3e7fa5d9b21ffcaea0c20b38e0b06deeffb0b0` |
| todo | 119423 B | 46237 B | 38222 B | 41269 B | `54419f7df8c972241639453557c481da264ba35b45c96e1e2b8b5b3769379d22` |
| dashboard | 169794 B | 63522 B | 51527 B | 55534 B | `ccf1115134115770a3d2e18b3eb141394090c5e27e972021ef6fc88eec407cd2` |
| context | 116313 B | 43907 B | 35932 B | 38714 B | `74343c753dd9dfb36f3743856069c33d67b53990ee1d0ce0ceda29165015023d` |
| virtualized | 123248 B | 47631 B | 38998 B | 42209 B | `7265158603003d6ef0bb15034e48047cb757066689abac23c8497252d8383930` |
| multipackage | 95340 B | 37501 B | 31020 B | 33510 B | `a908e6fbe0b0efdffe7d50dcb9a606b46d951866794640f10212d9d15aeecdbd` |
| cmdapp | 95358 B | 37516 B | 31121 B | 33494 B | `5ec1dcb73d0e4c85abe9d6549cd8305934914ae034c2fa0dc97798429a5865d9` |
| router | 116602 B | 44598 B | 36780 B | 39614 B | `64008632fe639c1c6851ecfa1517f5294b8feeb715b779b200a8a35e7a7dd5e0` |
| router-dashboard | 233415 B | 93591 B | 77001 B | 82143 B | `24a53d9c4546d3dbb6354cd414048787edf2143b14d0a9ca97167f4c6b091f14` |
| resource | 156239 B | 68172 B | 57464 B | 61009 B | `5075f6dd01b1ee7e78a0c96c576827805fd1f7c059c5aec3cd61426b96c4fcde` |

All base-to-final raw, gzip, Brotli, and Zstandard sizes and SHA-256 values are
unchanged. The three-cell budget decision is:

| app/format | measured | old budget | overage | new budget | headroom |
| --- | ---: | ---: | ---: | ---: | ---: |
| dashboard raw | 169794 B | 168960 B | 834 B | 171008 B | 1214 B |
| router-dashboard zstd | 82143 B | 81920 B | 223 B | 82944 B | 801 B |
| resource br | 57464 B | 57344 B | 120 B | 58368 B | 904 B |

Every other raw and compressed budget remains unchanged. Compression ratio
limits remain gzip `52.00%`, Brotli `38.00%`, and Zstandard `46.00%`.
This evidence-backed migration is the only supersession of the older Go
`1.22.12` toolchain source-of-truth recommendation.

## Controlled Select Correctness Rebaseline — 2026-07-31

The frozen base is `6bcbde2318099f92f940b817a69c2e1f52ad4058`.
The behavior-correct branch head is
`9531d48ae64fd0db966dad4e03c7f67575e6ed27`. Measurements use Go `1.26.5`,
TinyGo `0.41.1`, Linux amd64, gzip `1.14`, Brotli `1.2.0`, and Zstandard
`1.5.7`.

The accepted runtime guarantee restores a controlled single-value select's
current value after option reconciliation. It preserves browser ownership for
uncontrolled selects, stable select DOM identity, and the absence of synthetic
`input` or `change` events. Dedicated Chrome evidence passes under both
standard Go and TinyGo.

Two isolated frozen-base builds and two isolated behavior-correct-head builds
were byte-identical within each ref for raw, gzip, Brotli, and Zstandard
outputs. The resulting raw matrix is:

| app | frozen base | reviewed head | delta | reviewed raw SHA-256 | old raw budget result |
| --- | ---: | ---: | ---: | --- | --- |
| counter | 84536 B | 85052 B | +516 B | `33f72247577f8f2b6f5d6ee889e267d48548236e1cdca0208d9c46efdfbb91b1` | pass |
| components | 90176 B | 90691 B | +515 B | `8a94c41998279827c415d6f4e3fa16c4c74389f43f62d57a1e85a7278858c66c` | pass |
| todo | 119423 B | 119938 B | +515 B | `54b7308666ddb090bf5ae3a93120e1158a7fa218b0f8698ab78a5462b06e1664` | pass |
| dashboard | 169794 B | 170308 B | +514 B | `bc59482b4746f5a9100e8072769289d6e34d99fc57121c7358bce8e44afa2aa9` | pass |
| context | 116313 B | 116828 B | +515 B | `f4d4c19c691aa91a568fa361dc0fd017812fe1cb81c040443611627346168e8a` | fail by 92 B |
| virtualized | 123248 B | 123762 B | +514 B | `1c36299671986e8aef8c2203ccd6a5bb2159a2246677a1189483c3dd0efa5378` | pass |
| multipackage | 95340 B | 95854 B | +514 B | `83f3402aae1b8a0bd358616810e0574e0a6cf02ff3ef197cca47c5f38c78c3f0` | pass |
| cmdapp | 95358 B | 95872 B | +514 B | `f0ae215386be33e6b9f35b4e62a19a32cabca8d11c7aaa9689a0e5e5b66fcbae` | pass |
| router | 116602 B | 117117 B | +515 B | `f49b180f0a523a2ec5e3d5e9cf8152f799c3f8ffbcdf312e73516844c2792978` | fail by 381 B |
| router-dashboard | 233415 B | 233941 B | +526 B | `9dd248b23ebc527037eb53e504e9f90fd5bfa52cbcc94e45ea23943b8ed578d8` | fail by 469 B |
| resource | 156239 B | 156764 B | +525 B | `03f7f32e9c9e422a78b7aa8bf131d8575c4b0a8471be106adc1062d7703a2aa1` | fail by 92 B |

Deterministic compression used gzip `-n -9`, Brotli quality 11, and
Zstandard level 19. Every compressed size remained within its existing ceiling
and every gzip `52.00%`, Brotli `38.00%`, and Zstandard `46.00%` ratio limit
passed.

| app | gzip | br | zstd | result |
| --- | ---: | ---: | ---: | --- |
| counter | 34008 B | 28491 B | 30675 B | pass |
| components | 35899 B | 29739 B | 32074 B | pass |
| todo | 46336 B | 38399 B | 41370 B | pass |
| dashboard | 63874 B | 51504 B | 55609 B | pass |
| context | 44024 B | 35925 B | 38825 B | pass |
| virtualized | 47750 B | 39062 B | 42290 B | pass |
| multipackage | 37642 B | 31123 B | 33610 B | pass |
| cmdapp | 37651 B | 31289 B | 33610 B | pass |
| router | 44739 B | 36877 B | 39773 B | pass |
| router-dashboard | 93747 B | 77105 B | 82271 B | pass |
| resource | 68351 B | 57508 B | 61140 B | pass |

Four bounded private renderer shapes preserved the browser contract but failed
at least one mandatory raw sentinel ceiling:

| candidate | shape | counter | context | router | router-dashboard | resource | rejection |
| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| A | return and reuse normalized DOM props | 85013 B | 116789 B | 117087 B | 233877 B | 156700 B | four blockers remained |
| B | mount-only helper plus local patch synchronization | 84813 B | 116590 B | 116892 B | 233689 B | 156513 B | router cells remained over |
| C1 | compact value/presence handoff with local synchronization | 84715 B | 116492 B | 116788 B | 233584 B | 156408 B | router by 52 B; router-dashboard by 112 B |
| C2 | compact handoff plus shared synchronization | 84920 B | 116690 B | 116994 B | 233797 B | 156626 B | router cells remained over |

The decision is to preserve the simpler shared post-children runtime
implementation instead of adopting compiler-shaped Candidate C1 solely for a
smaller binary. The measured correctness cost is accepted through four aligned
one-KiB raw ceiling increases:

| app | measured | old ceiling | new ceiling | headroom |
| --- | ---: | ---: | ---: | ---: |
| context | 116828 B | 116736 B | 117760 B | 932 B |
| router | 117117 B | 116736 B | 117760 B | 643 B |
| router-dashboard | 233941 B | 233472 B | 234496 B | 555 B |
| resource | 156764 B | 156672 B | 157696 B | 932 B |

This rebaseline does not authorize general renderer optimization, compressed
budget changes, ratio changes, workflow changes, or unrelated application
headroom. No compressed ceiling or compression command changed.

## Controlled Select Descendant-Component Follow-up - 2026-07-31

The parent-driven correction is
`9531d48ae64fd0db966dad4e03c7f67575e6ed27`, and the initial size closeout is
`77802ebd219c89805cfc8e5f3c953854a5654055`. The descendant-component
follow-up is the commit containing this audit, `fix(runtime): sync selects
after child component updates`. It closes review finding `3690405967` without
changing the public API or the single-value select contract.

The pre-fix fixture rendered a controlled select with authored value `"b"` and
only option `"a"`. A stateful child then added option `"b"` independently. The
parent render count remained `9`, the child render count advanced from `9` to
`10`, and the same select DOM node contained options `["a", "b"]`, but the
browser value remained `"a"` at index `0`. No `input` or `change` event fired.
The same failure reproduced under standard Go and TinyGo.

The correction retains the nearest mounted select on mounted component nodes,
propagates that owner through elements, fragments, and nested components, and
reapplies the select's current mounted VNode props after a successful
independent child patch. A nested select shadows an outer owner, and release
clears retained ownership before component teardown. The fixture also changes
the parent value from `"b"` to `"c"` before a child-only option insertion; the
result selects current value `"c"`, proving that no authored value was captured
at mount time. Ten fresh package/server/Chrome runs passed under standard Go,
and ten passed under TinyGo, with stable DOM identity and no synthetic events.

Two bounded compact variants were already exhausted alongside the clearer
ownership implementation:

| candidate | resource gzip | old-ceiling result | decision |
| --- | ---: | ---: | --- |
| compact variant 1 | 68629 B | 21 B over | rejected; the smaller compiler shape did not clear the gate |
| clear nearest-select ownership | 68636 B | 28 B over | accepted for explicit ownership and lifecycle behavior |
| compact variant 2 | 68637 B | 29 B over | rejected; no correctness or budget advantage |

Measurements compare the initial closeout head with the accepted follow-up in
the same extraction path using Go `1.26.5`, TinyGo `0.41.1`, Linux amd64, gzip
`1.14`, Brotli `1.2.0`, and Zstandard `1.5.7`. Compression uses the unchanged
budget commands: gzip level 9, Brotli quality 11, and Zstandard level 19.

| app | format | current head | follow-up | delta | ceiling | headroom |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| counter | raw | 85052 B | 85439 B | +387 B | 97280 B | 11841 B |
| counter | gzip | 34008 B | 34339 B | +331 B | 56320 B | 21981 B |
| counter | br | 28491 B | 28633 B | +142 B | 40960 B | 12327 B |
| counter | zstd | 30675 B | 30823 B | +148 B | 49152 B | 18329 B |
| components | raw | 90691 B | 91078 B | +387 B | 107520 B | 16442 B |
| components | gzip | 35899 B | 36104 B | +205 B | 56320 B | 20216 B |
| components | br | 29739 B | 29845 B | +106 B | 43008 B | 13163 B |
| components | zstd | 32074 B | 32261 B | +187 B | 49152 B | 16891 B |
| todo | raw | 119938 B | 120344 B | +406 B | 122880 B | 2536 B |
| todo | gzip | 46336 B | 46571 B | +235 B | 56320 B | 9749 B |
| todo | br | 38399 B | 38506 B | +107 B | 40960 B | 2454 B |
| todo | zstd | 41370 B | 41514 B | +144 B | 49152 B | 7638 B |
| dashboard | raw | 170308 B | 170706 B | +398 B | 171008 B | 302 B |
| dashboard | gzip | 63874 B | 64126 B | +252 B | 71680 B | 7554 B |
| dashboard | br | 51504 B | 51645 B | +141 B | 53248 B | 1603 B |
| dashboard | zstd | 55609 B | 55772 B | +163 B | 61440 B | 5668 B |
| context | raw | 116828 B | 117215 B | +387 B | 117760 B | 545 B |
| context | gzip | 44024 B | 44232 B | +208 B | 46080 B | 1848 B |
| context | br | 35925 B | 36098 B | +173 B | 36864 B | 766 B |
| context | zstd | 38825 B | 38961 B | +136 B | 40960 B | 1999 B |
| virtualized | raw | 123762 B | 124160 B | +398 B | 124928 B | 768 B |
| virtualized | gzip | 47750 B | 47986 B | +236 B | 49152 B | 1166 B |
| virtualized | br | 39062 B | 39122 B | +60 B | 40960 B | 1838 B |
| virtualized | zstd | 42290 B | 42422 B | +132 B | 44032 B | 1610 B |
| multipackage | raw | 95854 B | 96260 B | +406 B | 110592 B | 14332 B |
| multipackage | gzip | 37642 B | 37856 B | +214 B | 56320 B | 18464 B |
| multipackage | br | 31123 B | 31354 B | +231 B | 43008 B | 11654 B |
| multipackage | zstd | 33610 B | 33783 B | +173 B | 49152 B | 15369 B |
| cmdapp | raw | 95872 B | 96278 B | +406 B | 110592 B | 14314 B |
| cmdapp | gzip | 37651 B | 37861 B | +210 B | 56320 B | 18459 B |
| cmdapp | br | 31289 B | 31394 B | +105 B | 43008 B | 11614 B |
| cmdapp | zstd | 33610 B | 33797 B | +187 B | 49152 B | 15355 B |
| router | raw | 117117 B | 117523 B | +406 B | 117760 B | 237 B |
| router | gzip | 44739 B | 44971 B | +232 B | 58368 B | 13397 B |
| router | br | 36877 B | 37005 B | +128 B | 45056 B | 8051 B |
| router | zstd | 39773 B | 39933 B | +160 B | 51200 B | 11267 B |
| router-dashboard | raw | 233941 B | 234357 B | +416 B | 234496 B | 139 B |
| router-dashboard | gzip | 93747 B | 93995 B | +248 B | 94208 B | 213 B |
| router-dashboard | br | 77105 B | 77381 B | +276 B | 77824 B | 443 B |
| router-dashboard | zstd | 82271 B | 82523 B | +252 B | 82944 B | 421 B |
| resource | raw | 156764 B | 157180 B | +416 B | 157696 B | 516 B |
| resource | gzip | 68351 B | 68636 B | +285 B | 69632 B | 996 B |
| resource | br | 57508 B | 57823 B | +315 B | 58368 B | 545 B |
| resource | zstd | 61140 B | 61371 B | +231 B | 61440 B | 69 B |

SHA-256 values identify the exact measured current-head and follow-up streams:

| app | current raw / gzip / br / zstd | follow-up raw / gzip / br / zstd |
| --- | --- | --- |
| counter | `33f72247577f8f2b6f5d6ee889e267d48548236e1cdca0208d9c46efdfbb91b1` / `59044087171271c521850c409618f608285d49814125f08920571d63b5a51880` / `0005f228978e0157b5827b850a88214374c1dcac621754fd9d78fe98a5a425b9` / `b2ad4989f07a3f565789946d819a7c0641d78c2c556e54c3686be2cc7f035cbc` | `773b4effb417d4497f051387e0c2221206014f6cdc79f37cc719f47b0ce8d10f` / `6709d97e54f31ca0e803815cf79e21e80ac3845113426111fc6c6fd43cebe3dc` / `e228bb0e1a8c9e0fd72e0fc316e0f84675332010b7d6d17f82b6c23ad859bb9a` / `bf67579f1d22115f936c32299c57a963ac7a52f9f62b77e77da246535753ea0a` |
| components | `8a94c41998279827c415d6f4e3fa16c4c74389f43f62d57a1e85a7278858c66c` / `f74455980d64d9cc36705d517af8e08a054c6601fcad211efa6342c2e08598ac` / `96a0c417639f7462ee53cb26afb42e6d1e625854a6961ecca870fade80f4fd40` / `e1cfd1f7e2f47c56b8f7a6aeafff066dfd0979e353b9c960891b039c20a14386` | `e29e56e0ff8a863f35faf5423a5ea0e888e97c244ef4cd87d420fbf9818117b7` / `2821267b0fb2ec2857fff5a95625a9a8d92f939172416a8846bd26b410123a14` / `bb5dceee395bfe0828b1d44e64ca66dcd8ba7cc92ecd04f66e6df78682de4ee4` / `4a38d62198fd72b7516f00ea0ad5adc7939014de9768d22bf071ed91039b728d` |
| todo | `54b7308666ddb090bf5ae3a93120e1158a7fa218b0f8698ab78a5462b06e1664` / `f3ac9b9b13177ab7916f0f426ea93f21d74587341c09b6dc1708f4a197b766ac` / `044130a727035557237045b19e1fa8f5637361eb68805a1e4bbfefcf8d795042` / `2ce6d7540b09311ae9a01d0c5770612ffefd500fdaaffb13437a778843e966c3` | `96c27f77d529548334fc683e4a8a847830f742a2a44a9c53cd143cd3328fcde4` / `187305a1e3b12e3fe994704ecf023f8d4da884abcc98dbb743d78eaec4ddbe70` / `ec67c7bcb0aeffd4cf3bc0353dadb9062ab0d7889430f2bac9ae44d73d6071aa` / `c67daf7da9c9a0e0780e25460038c217523c31c85d84946835ef54039d5c8842` |
| dashboard | `bc59482b4746f5a9100e8072769289d6e34d99fc57121c7358bce8e44afa2aa9` / `f651da4454df46f897b508da4721dc10d90f0c6f375ed3e17be3882c169c9553` / `bc4a86bd158acfd4759dd95f37b754fafc0823b8e856448ded97c15f2bd63546` / `95466f901011a6eec1cbb52cb19f1d249ec9256344cd0520dd0589ff3158dec8` | `bbbef9fbd418fe85746d3544286b2457b1ece002cf776ceb20afbffebaca2864` / `837fc55e52a99acc698fa977300d93b95f9b673d8b84d1a7e576a3112592d15` / `a9ab0f3b7641082ccac7f22460e75d5cf791c439d125c6d05af0bb2c2bfd747d` / `5d2310ade93819eba619ad64fe0d8a11dd141b565e0c694ebfe0549910901d02` |
| context | `f4d4c19c691aa91a568fa361dc0fd017812fe1cb81c040443611627346168e8a` / `f17e3cf16f1e0926a8948ec4983d362a4d48b83043510842b399a8a025908209` / `48b37204950035db1db84074931603a8682e95ba1e5b65cdd2b37737cfee0e5b` / `edeefab63505a649e51ea9b8e96100f384dbd1a6d92ed131a9e8a63eaa7ebe1a` | `a7a0691659447a67cbbdf183f198107b86a09be25e99d4b589a0c4ef811f2067` / `f1deba745e4b669df669a0961fb110b65376738169fb1579489208a74c5b1026` / `e04340ff513e8e1a9d4088e7e9dbb8ae8472085b3b58811f3f83356f921a3493` / `b94940a80d59510b3f3677becfbc15de4bd5a57586cb50c87845a776ed639b5b` |
| virtualized | `1c36299671986e8aef8c2203ccd6a5bb2159a2246677a1189483c3dd0efa5378` / `eaecb7cb68d70d7e90a6ba1f3f38c0a838d65e5bf7053d90158b8be3cf1f21d5` / `000bcfcabcb6ab7b4a50850232f960c7522ff6b111b5cf8420d37cb88df1697d` / `9ea8454dfe3fd8541e2db854b04923830e3d30ba0971117552775d76cd62d952` | `4e120d473883ba074285a918b73e1b37a66f81e7850aa7d7b985f2c8cc8d263b` / `cc19b8c53c707468ae35dd700224aa02a57054fc2fc222bab6aef979fa6a8868` / `e875b23552a8b4542298e8e9bbf9d1150dcedeab76e743aa3cb16077a513b42a` / `1d421a384b4477ca373449dfe4deb233ed8299b5f78eb5c9e19172b0f2db203f` |
| multipackage | `83f3402aae1b8a0bd358616810e0574e0a6cf02ff3ef197cca47c5f38c78c3f0` / `5168bc907547ff19bf1fb4b2da17ef74ce7fb80e4a4adc43d531a9a593e0a0be` / `ab69df4744ec8fa2686b87f3e731237cca8c86d7b15f3b06cc12b958aa7bd5ea` / `9e0dea0bdb28e893e2374e0eed8ab107f4cd350dc0b2a256b56de0a41f222f69` | `601d36f8d6e5b90bffcb80e164c1c8f73502df42a8a20dd017ea16dec5e37677` / `404e672b5581c747265a97ff8c0eb443979438a5f1cbd061d990273f28385b16` / `cc3631ccbd7e474d9aaa0fd09efdbec755935af6f19c33d76128a2f68708344d` / `697693ce66d0af59d5ade0531d17b32c228a2baa57641aadbb7f4ca1572f9d85` |
| cmdapp | `f0ae215386be33e6b9f35b4e62a19a32cabca8d11c7aaa9689a0e5e5b66fcbae` / `6cacb0f27327532d7960fb32d0a1d974de16e0f98f43ccb5032b63081f584933` / `0fcc6c7990aaeefe0aaa98583d0bca0e6935b7a1008789c1979151cedd863158` / `f2c14a052573f66c609257e59faf5fdb841e510ab86b9fc37cda8de77e91dd0c` | `32a47fc52c53c53de985602a4624cfe67f76fd6aa444a8f61823389fe9648640` / `8704fa998501e2ae9b13867a284e7ecf0fd9fad41018df80a4fa942caafa1216` / `eae473813325102d777a136074ba82b5da04c54587017a46627708546b73b9f2` / `e7428a7d96e222a9566ce048065000712b5790e9f7601dddc2c46756002bd598` |
| router | `f49b180f0a523a2ec5e3d5e9cf8152f799c3f8ffbcdf312e73516844c2792978` / `3320ef65841ccbf4af2001b08449560cce3f05f16459e6df070fb9b02bd9675d` / `f1ff16268f064313e9fffdf8be7acfaa0e4ba829c5250e2a3724eac4e89729c4` / `18d7a50a16fed4f47d16c06ed27376483db5b3429bdd531ee9546dc0f27c67b6` | `242c0f6c65028f3f21a00932265d5c24148df2c1b84cd8d76bad267fcdcc6638` / `cbfd9e5a9f5bff9c29bdf3a14e97e42cc308aecd3e9b79196ce94ee43db2efdf` / `db0e79ce2ef23b5c7f81baa0cec76756b9894a72487da468efef5b3c86e08702` / `17ef65a118cba5daa930be86cb3a806c5a8f9a0da1c609b830659502c8b6539b` |
| router-dashboard | `9dd248b23ebc527037eb53e504e9f90fd5bfa52cbcc94e45ea23943b8ed578d8` / `4520eae8500d12a21f0f0aca562bf88d26e817d45a3ecb683e95a7dab40458ac` / `f83cf0932ae31a132e475e9b7c4c07c445699e3a8bf33ceb0a716cc50d11d504` / `18dc21f40c6e21e1808e2bd2f22c558d16249a3a0ba511c1a276381bf9c9a782` | `4ca08e0be2db602601306e1b7c7f2ad48454f39c57b7cd798f2268df4cc0ae97` / `6214017b749fa20c2ac72d17230ee7e7322af29110d66dc1b59a62fa6b1f94fc` / `9734a0d25a87000a92161f230fa85b5f612f0dbf41f1f0448b5d9e859a9baa35` / `327781d1ca426e1d0d36a60ddde288894823f83b8ca3289463e99afc0a8ad3a5` |
| resource | `03f7f32e9c9e422a78b7aa8bf131d8575c4b0a8471be106adc1062d7703a2aa1` / `bca9aad94cc3a2bcaf283d930de0c8d66ac84db66b78518cae27f8fd06a7f9e1` / `161223db5a6cfd3cc027207540e38c483a86fe949cd6955244bf45915b1259ca` / `ce9ce0fd7033c93514cbaa65edf06af4a3e568b9de6cd1e33d007f94a23a3401` | `17b29f7b316e77d5b5872f613f3699db43bb0de4122c49c71632af6107a0a171` / `5f5c0187a2ff7b488d8d60a14744348c9b1aa7bcce13da959ac96ebe7616435e` / `97d2966701e8c746d73f6ae72b252a88ce4d5c100a83db72bae935e519d2a560` / `49c3731eccf30634e1cc1affa9a32cdf39d787e2924712de95289234fb43b376` |

Every final raw artifact reproduced byte-for-byte in a second package tree, and
all compressed sizes reproduced. All 44 cells pass. The only ceiling change is
`resource` gzip, from `68608 B` to `69632 B`, leaving `996 B` of headroom.
Raw, Brotli, and Zstandard ceilings are unchanged, as are gzip `52.00%`,
Brotli `38.00%`, and Zstandard `46.00%` ratio limits and the WASM workflow.

## Context Topology Recovery — 2026-08-01

This measurement compares frozen base
`195ccf8b9bf3300b7bd86bbcea07ae9dbd727c2a` with measured implementation head
`9cc93493b619bd570041fe5512f9ef81e2eeb063`. The documentation closeout is the
commit containing this section; it does not change runtime or package inputs.
The runtime correction rebinds a context subscription before evaluating a
selector against a newly nearest provider.

Measurements use Go `1.26.5`, TinyGo `0.41.1`, Linux amd64, gzip `1.14`,
Brotli `1.2.0`, and Zstandard `1.5.7`. Both refs were built sequentially in the
same extraction path with the CI package order and shared task-owned caches.
Compression uses the unchanged budget commands: gzip level 9, Brotli quality
11, and Zstandard level 19. SHA comparison fixes the source mtime for comparable
gzip container metadata; the compression command and measured byte count are
unchanged.

| app | raw base → final | gzip base → final | br base → final | zstd base → final |
| --- | ---: | ---: | ---: | ---: |
| counter | 85439 → 85439 B (0 B) | 34348 → 34348 B (0 B) | 28633 → 28633 B (0 B) | 30823 → 30823 B (0 B) |
| components | 91078 → 91078 B (0 B) | 36113 → 36113 B (0 B) | 29845 → 29845 B (0 B) | 32261 → 32261 B (0 B) |
| todo | 120344 → 120344 B (0 B) | 46580 → 46580 B (0 B) | 38506 → 38506 B (0 B) | 41514 → 41514 B (0 B) |
| dashboard | 170706 → 170706 B (0 B) | 64135 → 64135 B (0 B) | 51645 → 51645 B (0 B) | 55772 → 55772 B (0 B) |
| context | 117215 → 117209 B (-6 B) | 44241 → 44252 B (+11 B) | 36098 → 36066 B (-32 B) | 38961 → 38957 B (-4 B) |
| virtualized | 124160 → 124160 B (0 B) | 47995 → 47995 B (0 B) | 39122 → 39122 B (0 B) | 42422 → 42422 B (0 B) |
| multipackage | 96260 → 96260 B (0 B) | 37865 → 37865 B (0 B) | 31354 → 31354 B (0 B) | 33783 → 33783 B (0 B) |
| cmdapp | 96278 → 96278 B (0 B) | 37870 → 37870 B (0 B) | 31394 → 31394 B (0 B) | 33797 → 33797 B (0 B) |
| router | 117523 → 117523 B (0 B) | 44980 → 44980 B (0 B) | 37005 → 37005 B (0 B) | 39933 → 39933 B (0 B) |
| router-dashboard | 234357 → 234351 B (-6 B) | 94004 → 94011 B (+7 B) | 77381 → 77333 B (-48 B) | 82523 → 82516 B (-7 B) |
| resource | 157180 → 157180 B (0 B) | 68645 → 68645 B (0 B) | 57823 → 57823 B (0 B) | 61371 → 61371 B (0 B) |

Final ceilings and byte headroom remain:

| app | raw final / ceiling (headroom) | gzip final / ceiling (headroom) | br final / ceiling (headroom) | zstd final / ceiling (headroom) |
| --- | ---: | ---: | ---: | ---: |
| counter | 85439 / 97280 B (11841 B) | 34348 / 56320 B (21972 B) | 28633 / 40960 B (12327 B) | 30823 / 49152 B (18329 B) |
| components | 91078 / 107520 B (16442 B) | 36113 / 56320 B (20207 B) | 29845 / 43008 B (13163 B) | 32261 / 49152 B (16891 B) |
| todo | 120344 / 122880 B (2536 B) | 46580 / 56320 B (9740 B) | 38506 / 40960 B (2454 B) | 41514 / 49152 B (7638 B) |
| dashboard | 170706 / 171008 B (302 B) | 64135 / 71680 B (7545 B) | 51645 / 53248 B (1603 B) | 55772 / 61440 B (5668 B) |
| context | 117209 / 117760 B (551 B) | 44252 / 46080 B (1828 B) | 36066 / 36864 B (798 B) | 38957 / 40960 B (2003 B) |
| virtualized | 124160 / 124928 B (768 B) | 47995 / 49152 B (1157 B) | 39122 / 40960 B (1838 B) | 42422 / 44032 B (1610 B) |
| multipackage | 96260 / 110592 B (14332 B) | 37865 / 56320 B (18455 B) | 31354 / 43008 B (11654 B) | 33783 / 49152 B (15369 B) |
| cmdapp | 96278 / 110592 B (14314 B) | 37870 / 56320 B (18450 B) | 31394 / 43008 B (11614 B) | 33797 / 49152 B (15355 B) |
| router | 117523 / 117760 B (237 B) | 44980 / 58368 B (13388 B) | 37005 / 45056 B (8051 B) | 39933 / 51200 B (11267 B) |
| router-dashboard | 234351 / 234496 B (145 B) | 94011 / 94208 B (197 B) | 77333 / 77824 B (491 B) | 82516 / 82944 B (428 B) |
| resource | 157180 / 157696 B (516 B) | 68645 / 69632 B (987 B) | 57823 / 58368 B (545 B) | 61371 / 61440 B (69 B) |

Nine applications have identical raw, gzip, Brotli, and Zstandard SHA-256
values across the comparison. The two changed streams are:

| app | base raw / gzip / br / zstd SHA-256 | final raw / gzip / br / zstd SHA-256 |
| --- | --- | --- |
| context | `a7a0691659447a67cbbdf183f198107b86a09be25e99d4b589a0c4ef811f2067` / `fb11ea8f199d8636557e9ab80ed38f11d5e7fc5b3561363095442e7981ad9a0e` / `e04340ff513e8e1a9d4088e7e9dbb8ae8472085b3b58811f3f83356f921a3493` / `b94940a80d59510b3f3677becfbc15de4bd5a57586cb50c87845a776ed639b5b` | `547722e8a94f126cf5f21944b9b120e31aae023e2cb40fbbced1fccf9796c626` / `b885e8372bd6373b07c5862f797eb801bb6192496a6804b0cb6f1d9cbe358b72` / `504a2b1ec41b988dbad31915260ff5c6d6e09dd539dfa77b78e8b47b26b98822` / `1213d9c6fa431403291a36babd86b7e70d9bd8e7edae60775c43bf46ae29461e` |
| router-dashboard | `4ca08e0be2db602601306e1b7c7f2ad48454f39c57b7cd798f2268df4cc0ae97` / `c69dad8280b65b1bccd1111c5fd7586d03922f240a9b722504c8ff9df9fd9a1b` / `9734a0d25a87000a92161f230fa85b5f612f0dbf41f1f0448b5d9e859a9baa35` / `327781d1ca426e1d0d36a60ddde288894823f83b8ca3289463e99afc0a8ad3a5` | `b00e2dcf4cdf997f7240c6b7efe475f6a8920715cfdbecbabe7105ba830c03d0` / `aa144e4726f4052fcac6e112bd5bb154f6ff0b102ea43ffff4ae274c16f52220` / `1f17b59c8157df476ce6214acf365001ed741f533f1840dfeb8cc0e4ef4e5b4d` / `305feb072e7b2a72d6b856b52d9de59fa008f29f011f223c9dcfcdc393ac6881` |

All 44 cells and all ratio limits pass. Gzip remains capped at `52.00%`,
Brotli at `38.00%`, and Zstandard at `46.00%`. No ceiling, ratio, compression
command, size workflow, or budget-matrix membership changed; the private
standard-Go browser fixture is not part of the size matrix.

## Repeated Mount Root Ownership — 2026-08-01

This measurement compares frozen base
`01ac132c10fc82decebe330e359027a92aeea68e` with package-input head
`5154edf5ec780f0e354209871f6baaad301af56a`. The documentation closeout is the
commit containing this section; it does not change runtime or package inputs.
The runtime correction removes the previous GoFrame-owned DOM range when a
later `Mount` transfers the single active application to the same or a
different root.

Measurements use Go `1.26.5`, TinyGo `0.41.1`, Linux amd64, gzip `1.14`,
Brotli `1.2.0`, and Zstandard `1.5.7`. Both refs were built sequentially in the
same extraction path with the CI package order and shared task-owned caches.
Compression uses the unchanged budget commands: gzip level 9, Brotli quality
11, and Zstandard level 19. SHA comparison fixes the source mtime for comparable
gzip container metadata; the compression command and measured byte count are
unchanged.

| app | raw base → final | gzip base → final | br base → final | zstd base → final |
| --- | ---: | ---: | ---: | ---: |
| counter | 85439 → 85481 B (+42 B) | 34348 → 34300 B (-48 B) | 28633 → 28650 B (+17 B) | 30823 → 30793 B (-30 B) |
| components | 91078 → 91120 B (+42 B) | 36113 → 36045 B (-68 B) | 29845 → 29966 B (+121 B) | 32261 → 32253 B (-8 B) |
| todo | 120344 → 120386 B (+42 B) | 46580 → 46537 B (-43 B) | 38506 → 38568 B (+62 B) | 41514 → 41485 B (-29 B) |
| dashboard | 170706 → 170748 B (+42 B) | 64135 → 64068 B (-67 B) | 51645 → 51669 B (+24 B) | 55772 → 55755 B (-17 B) |
| context | 117209 → 117251 B (+42 B) | 44252 → 44187 B (-65 B) | 36066 → 36107 B (+41 B) | 38957 → 38934 B (-23 B) |
| virtualized | 124160 → 124202 B (+42 B) | 47995 → 47963 B (-32 B) | 39122 → 39277 B (+155 B) | 42422 → 42412 B (-10 B) |
| multipackage | 96260 → 96302 B (+42 B) | 37865 → 37792 B (-73 B) | 31354 → 31414 B (+60 B) | 33783 → 33798 B (+15 B) |
| cmdapp | 96278 → 96320 B (+42 B) | 37870 → 37798 B (-72 B) | 31394 → 31383 B (-11 B) | 33797 → 33749 B (-48 B) |
| router | 117523 → 117565 B (+42 B) | 44980 → 44935 B (-45 B) | 37005 → 36992 B (-13 B) | 39933 → 39915 B (-18 B) |
| router-dashboard | 234351 → 234395 B (+44 B) | 94011 → 93964 B (-47 B) | 77333 → 77318 B (-15 B) | 82516 → 82494 B (-22 B) |
| resource | 157180 → 157224 B (+44 B) | 68645 → 68565 B (-80 B) | 57823 → 57768 B (-55 B) | 61371 → 61404 B (+33 B) |

Final ceilings and byte headroom remain:

| app | raw final / ceiling (headroom) | gzip final / ceiling (headroom) | br final / ceiling (headroom) | zstd final / ceiling (headroom) |
| --- | ---: | ---: | ---: | ---: |
| counter | 85481 / 97280 B (11799 B) | 34300 / 56320 B (22020 B) | 28650 / 40960 B (12310 B) | 30793 / 49152 B (18359 B) |
| components | 91120 / 107520 B (16400 B) | 36045 / 56320 B (20275 B) | 29966 / 43008 B (13042 B) | 32253 / 49152 B (16899 B) |
| todo | 120386 / 122880 B (2494 B) | 46537 / 56320 B (9783 B) | 38568 / 40960 B (2392 B) | 41485 / 49152 B (7667 B) |
| dashboard | 170748 / 171008 B (260 B) | 64068 / 71680 B (7612 B) | 51669 / 53248 B (1579 B) | 55755 / 61440 B (5685 B) |
| context | 117251 / 117760 B (509 B) | 44187 / 46080 B (1893 B) | 36107 / 36864 B (757 B) | 38934 / 40960 B (2026 B) |
| virtualized | 124202 / 124928 B (726 B) | 47963 / 49152 B (1189 B) | 39277 / 40960 B (1683 B) | 42412 / 44032 B (1620 B) |
| multipackage | 96302 / 110592 B (14290 B) | 37792 / 56320 B (18528 B) | 31414 / 43008 B (11594 B) | 33798 / 49152 B (15354 B) |
| cmdapp | 96320 / 110592 B (14272 B) | 37798 / 56320 B (18522 B) | 31383 / 43008 B (11625 B) | 33749 / 49152 B (15403 B) |
| router | 117565 / 117760 B (195 B) | 44935 / 58368 B (13433 B) | 36992 / 45056 B (8064 B) | 39915 / 51200 B (11285 B) |
| router-dashboard | 234395 / 234496 B (101 B) | 93964 / 94208 B (244 B) | 77318 / 77824 B (506 B) | 82494 / 82944 B (450 B) |
| resource | 157224 / 157696 B (472 B) | 68565 / 69632 B (1067 B) | 57768 / 58368 B (600 B) | 61404 / 61440 B (36 B) |

SHA-256 values identify every measured base and final stream:

| app | base raw / gzip / br / zstd SHA-256 | final raw / gzip / br / zstd SHA-256 |
| --- | --- | --- |
| counter | `773b4effb417d4497f051387e0c2221206014f6cdc79f37cc719f47b0ce8d10f` / `575f652628e05223ccf585abdc2b1c9680529870a0e5be0c159e344d5937a01d` / `e228bb0e1a8c9e0fd72e0fc316e0f84675332010b7d6d17f82b6c23ad859bb9a` / `bf67579f1d22115f936c32299c57a963ac7a52f9f62b77e77da246535753ea0a` | `1b4496b361964f115673be69193569e00c8f0783e4ffb0246192c956b5896457` / `44fc8c4c4ef1ca4dad2d0fc1683c3ce56e12f543e8181b80d509c0b02a44949f` / `63d3868c49fca10129296cd0c948eec7100d9bc09355676a6326e93ce4ab6d58` / `0c7ebba2ac9804b1098d8bc3b2d54d252cb449f19e1a43ecc66f24aca55b1177` |
| components | `e29e56e0ff8a863f35faf5423a5ea0e888e97c244ef4cd87d420fbf9818117b7` / `26bbe0c559d25878b045c776d479e1dbe2b1012859c44189e830046039ec6b10` / `bb5dceee395bfe0828b1d44e64ca66dcd8ba7cc92ecd04f66e6df78682de4ee4` / `4a38d62198fd72b7516f00ea0ad5adc7939014de9768d22bf071ed91039b728d` | `287c9c495fbefbeb9bc7b11712d9baedef80b6d92c43ed5669be8602d8392c99` / `17798d14bb75c645d06f9cf2054be7dd0b63a63288288d2ae82d65be78eed43b` / `a7c0ecacbd442fee675d520e4523b2aab11582cb10053ec635530845cad5bac5` / `190a851c4c070dac803b73498f82f25c8090584673473440217822ad9c76eed2` |
| todo | `96c27f77d529548334fc683e4a8a847830f742a2a44a9c53cd143cd3328fcde4` / `ece4aa57b8ef90f7c3f4e094466449e142f3ec0f581155b8e86613be043391fa` / `ec67c7bcb0aeffd4cf3bc0353dadb9062ab0d7889430f2bac9ae44d73d6071aa` / `c67daf7da9c9a0e0780e25460038c217523c31c85d84946835ef54039d5c8842` | `132c288a9b0b418a378df200918e06500a8b8a3a29f84d68bf0311c624006d14` / `13488f66500cddb8364373237f2b7c28ab55f98248a07c8c8080f1459b545369` / `b64e8508907a3a620e4bd19831162a20b8c06f5f3243b0cc73821e0bde796398` / `a4a1dad9e3da988c71825919692941ef50d283c11ddcbd7a87155d79859c0eb2` |
| dashboard | `bbbef9fbd418fe85746d3544286b2457b1ece002cf776ceb20afbffebaca2864` / `75cacd7308ce086f2e0d57b1ce7726fc900ce174d52670ea19ff8b2b40f1b540` / `a9ab0f3b7641082ccac7f22460e75d5cf791c439d125c6d05af0bb2c2bfd747d` / `5d2310ade93819eba619ad64fe0d8a11dd141b565e0c694ebfe0549910901d02` | `03637d8cf568577acecc09012a13a37c74e948ef9238dec08221f209f7d2440d` / `7e0d77e74cb9bc00388499594f9c595359bedc62b950938ae2a969b76811404c` / `b3eaf0e24d5332de95d76c3d6f0bd88222cc47f097dcff59862d95f5a09a3d22` / `f5b0913a58e714432ef54b58b89494fdbadb22a4ef33621792e4cf25c56db320` |
| context | `547722e8a94f126cf5f21944b9b120e31aae023e2cb40fbbced1fccf9796c626` / `b885e8372bd6373b07c5862f797eb801bb6192496a6804b0cb6f1d9cbe358b72` / `504a2b1ec41b988dbad31915260ff5c6d6e09dd539dfa77b78e8b47b26b98822` / `1213d9c6fa431403291a36babd86b7e70d9bd8e7edae60775c43bf46ae29461e` | `cf35b1eea5b6e4269326eea4948e21f6415643aa7265777bc84526b322e31fcd` / `737fde984ac35e45a4acfb07a4cd4906ddc46933a00d483cac468450c183c956` / `397eb36a67c47dc1b93376d821551c880c22ee76e92e8bc464793177e9efabdb` / `b532e9fc3b4e6d078c8b476ec95ef8e2fd4fba89ce0e5cda247746f6097bee23` |
| virtualized | `4e120d473883ba074285a918b73e1b37a66f81e7850aa7d7b985f2c8cc8d263b` / `6b9cefa8bc0f2e72f913fdd0daa698c02c436ff1d4c5442da79c1cd8c89d729f` / `e875b23552a8b4542298e8e9bbf9d1150dcedeab76e743aa3cb16077a513b42a` / `1d421a384b4477ca373449dfe4deb233ed8299b5f78eb5c9e19172b0f2db203f` | `01899b2c6ac85fe4421299921b3f960de8aa581698a9d5b9c6d8e7d5c105c92b` / `197384ab681d1947b31abea26503c1d71781f2c688728096a83700cecc58d2e6` / `b7cfd6a391e4d273ba726af3c76c41c19ed0fd78c2ab22522335c3829e090592` / `304690e8391b4e63e68654283296cc554f886380344e1e470412496c0e9f0754` |
| multipackage | `601d36f8d6e5b90bffcb80e164c1c8f73502df42a8a20dd017ea16dec5e37677` / `7570bc260b95e4682c83d1ac5dea41a7a3b1b35efe8ad998a2b7d6ec3bfd3159` / `cc3631ccbd7e474d9aaa0fd09efdbec755935af6f19c33d76128a2f68708344d` / `697693ce66d0af59d5ade0531d17b32c228a2baa57641aadbb7f4ca1572f9d85` | `40616cedda7a2f644cf80c5b17b883bee56632be8e4842699c56d7651f2199ba` / `2d894763d983abd041a88e5c6aea73bb6b0d6e2672a733002efb84baf5579c5e` / `e40bc2c465e3f64db1a0e99f2138d5e3221aa7001ce927d8b1dd3e5aea71a946` / `51453d580001c593059068056207d9c4fbdb14f36867861f1f3f3a0f7f036b22` |
| cmdapp | `32a47fc52c53c53de985602a4624cfe67f76fd6aa444a8f61823389fe9648640` / `4f91cc87eb05e8578069ef383bf05bb2b8038d9427370ca14717e661e9c6b3a5` / `eae473813325102d777a136074ba82b5da04c54587017a46627708546b73b9f2` / `e7428a7d96e222a9566ce048065000712b5790e9f7601dddc2c46756002bd598` | `4ee342784b97bb2e50cf1830959bfa45ba8304155c2c384ececce22f11181eb7` / `b89bd931c2a308a28943ed066636252cf2fa21652a7a1396c0c44d8b5a63b652` / `61fff06eeaa56f852a9e082e85b7acf5ac49fa51839b1a92c0d88332de9e13e2` / `981549f73bfcfa587a3a050e653f85e69922f73d3b6826fad3fa62324d1dba7d` |
| router | `242c0f6c65028f3f21a00932265d5c24148df2c1b84cd8d76bad267fcdcc6638` / `2f5ef3168e7c0b991a7e49cc276eb35e89fc809950e63cfebe28469686174768` / `db0e79ce2ef23b5c7f81baa0cec76756b9894a72487da468efef5b3c86e08702` / `17ef65a118cba5daa930be86cb3a806c5a8f9a0da1c609b830659502c8b6539b` | `df159357d037b5242b0bd297aa3b8819080d3b14e6ec3926fed97c7d51c3ce99` / `f315f934a8d8c9c36f80deab6dc211ef3653dcbd7582b7688ffd94e00eca0aa5` / `2a2742d8b30a38d9db438e26ec64569ed995f3badd72c9a02271a12fc3cf7315` / `43e7f3e522df944f16254e913260dbc8699c1c310303930dd0e917d20ca82758` |
| router-dashboard | `b00e2dcf4cdf997f7240c6b7efe475f6a8920715cfdbecbabe7105ba830c03d0` / `aa144e4726f4052fcac6e112bd5bb154f6ff0b102ea43ffff4ae274c16f52220` / `1f17b59c8157df476ce6214acf365001ed741f533f1840dfeb8cc0e4ef4e5b4d` / `305feb072e7b2a72d6b856b52d9de59fa008f29f011f223c9dcfcdc393ac6881` | `8cf8e96b377c9108404f36b8fe2bfa9216df428fb2ac34037262656902496892` / `894747320d642134bb0cfac46ffe6463316941a9af5d56d8350bc098c3a5856a` / `25cb0da315951534908265225993d1346ffefa09355f2785c4cd4d1390144be1` / `3c3529f718bcc2992438a335d3bdbef49c2437ce1d72093d6c15c4963239848c` |
| resource | `17b29f7b316e77d5b5872f613f3699db43bb0de4122c49c71632af6107a0a171` / `fe18f40401dacf06b37b1f15d59cd4ea32831998148d5d5df1172eb7878f9454` / `97d2966701e8c746d73f6ae72b252a88ce4d5c100a83db72bae935e519d2a560` / `49c3731eccf30634e1cc1affa9a32cdf39d787e2924712de95289234fb43b376` | `5827b43710546eb152e8430b22579053064ab4f9c2395b1122212f740e0e405a` / `f942eb412a62ee54a7612d14706fec9449bf07d0f513bcd7021abbb952de1601` / `2e9de7a76a7d403d99e2df9f24152e42f13467a7bccb2bb67535961b06788db5` / `e7758fb4623f32d04facd20148c4cdb2e07e02af1323c8384f66ad94a6b73720` |

All 44 cells and all ratio limits pass. Gzip remains capped at `52.00%`,
Brotli at `38.00%`, and Zstandard at `46.00%`. No ceiling, ratio, compression
command, size workflow, or budget-matrix membership changed; the private
repeated-mount browser fixture is not part of the size matrix.

## Repeated Mount Nested-Target Guard — 2026-08-02

This closeout compares reviewed repeated-Mount head
`d9c59a7b2e065cd9265108460579f028a8c7c02f` with package-input commit
`5ab337cc35bcdf5d4bd1ec9fc11e22581b9bb4f2`. At the reviewed head, mounting
into an application-owned descendant destroyed App A, ran its cleanups, removed
the target from the document, and mounted App B into the retained but
disconnected element. A host-owned descendant appended inside the current root
but outside the GoFrame-mounted range remained connected and could host the
replacement. The final contract intentionally rejects every different
descendant of the current root to keep the ownership rule simple, reviewable,
size-bounded, and independent of mounted-range traversal.

Four bounded implementations were measured. Candidate A traversed the mounted
range and added about `472 B` to `490 B` raw; Candidate B reduced ancestry to
direct children but added about `620 B` to `637 B` raw. Candidate C1 used the
whole current root inline and added about `251 B` to `256 B` raw. Candidate C2
moved the same whole-root condition into a private helper and added `216 B` to
`235 B` raw. C2 was selected because it is behaviorally complete, smaller than
C1, and does not change renderer, scheduler, teardown, or mounted-node
representation.

The selected condition is:

```go
mountedApp.tree != nil &&
	!root.Equal(mountedApp.root) &&
	mountedApp.root.Call("contains", root).Bool()
```

It runs after successful target lookup and before application teardown. The
current root and roots outside its subtree remain valid. A different
application-owned or host-owned descendant is rejected with
`goframe: cannot mount inside current root` while the current application is
still active. Standard-Go Chrome evidence covers both rejections, preserved DOM
identity and cleanup counts, and post-rejection interaction. TinyGo `0.41.1`
uses trap-style panic lowering, so the missing-root and descendant-panic cases
are not invoked there; all non-panic replacement and isolation scenarios pass
under TinyGo.

Measurements use Go `1.26.5`, TinyGo `0.41.1` with LLVM `20.1.1`, Linux amd64,
gzip `1.14`, Brotli `1.2.0`, and Zstandard `1.5.7`. Both refs were built
sequentially from the same absolute extraction path with the CI application
order and shared task-owned caches. Compression uses gzip level 9, Brotli
quality 11, and Zstandard level 19. Hash comparison uses the same
`bundle.00000000.wasm` source basename and a fixed 2000-01-01 source mtime;
these preserve the normative budget byte counts while removing path and mtime
differences from compressed-stream identity.

| app | raw reviewed → final | gzip reviewed → final | br reviewed → final | zstd reviewed → final |
| --- | ---: | ---: | ---: | ---: |
| counter | 85481 → 85716 B (+235 B) | 34300 → 34400 B (+100 B) | 28650 → 28747 B (+97 B) | 30793 → 30901 B (+108 B) |
| components | 91120 → 91355 B (+235 B) | 36045 → 36147 B (+102 B) | 29966 → 29989 B (+23 B) | 32253 → 32343 B (+90 B) |
| todo | 120386 → 120621 B (+235 B) | 46537 → 46725 B (+188 B) | 38568 → 38617 B (+49 B) | 41485 → 41587 B (+102 B) |
| dashboard | 170748 → 170964 B (+216 B) | 64068 → 64174 B (+106 B) | 51669 → 51777 B (+108 B) | 55755 → 55881 B (+126 B) |
| context | 117251 → 117486 B (+235 B) | 44187 → 44295 B (+108 B) | 36107 → 36184 B (+77 B) | 38934 → 39047 B (+113 B) |
| virtualized | 124202 → 124418 B (+216 B) | 47963 → 48053 B (+90 B) | 39277 → 39226 B (-51 B) | 42412 → 42489 B (+77 B) |
| multipackage | 96302 → 96537 B (+235 B) | 37792 → 37893 B (+101 B) | 31414 → 31412 B (-2 B) | 33798 → 33881 B (+83 B) |
| cmdapp | 96320 → 96555 B (+235 B) | 37798 → 37900 B (+102 B) | 31383 → 31428 B (+45 B) | 33749 → 33877 B (+128 B) |
| router | 117565 → 117800 B (+235 B) | 44935 → 45038 B (+103 B) | 36992 → 37057 B (+65 B) | 39915 → 40010 B (+95 B) |
| router-dashboard | 234395 → 234630 B (+235 B) | 93964 → 94041 B (+77 B) | 77318 → 77404 B (+86 B) | 82494 → 82605 B (+111 B) |
| resource | 157224 → 157459 B (+235 B) | 68565 → 68688 B (+123 B) | 57768 → 57813 B (+45 B) | 61404 → 61514 B (+110 B) |

Final ceilings and headroom are:

| app | raw final / ceiling (headroom) | gzip final / ceiling (headroom) | br final / ceiling (headroom) | zstd final / ceiling (headroom) |
| --- | ---: | ---: | ---: | ---: |
| counter | 85716 / 97280 B (11564 B) | 34400 / 56320 B (21920 B) | 28747 / 40960 B (12213 B) | 30901 / 49152 B (18251 B) |
| components | 91355 / 107520 B (16165 B) | 36147 / 56320 B (20173 B) | 29989 / 43008 B (13019 B) | 32343 / 49152 B (16809 B) |
| todo | 120621 / 122880 B (2259 B) | 46725 / 56320 B (9595 B) | 38617 / 40960 B (2343 B) | 41587 / 49152 B (7565 B) |
| dashboard | 170964 / 171008 B (44 B) | 64174 / 71680 B (7506 B) | 51777 / 53248 B (1471 B) | 55881 / 61440 B (5559 B) |
| context | 117486 / 117760 B (274 B) | 44295 / 46080 B (1785 B) | 36184 / 36864 B (680 B) | 39047 / 40960 B (1913 B) |
| virtualized | 124418 / 124928 B (510 B) | 48053 / 49152 B (1099 B) | 39226 / 40960 B (1734 B) | 42489 / 44032 B (1543 B) |
| multipackage | 96537 / 110592 B (14055 B) | 37893 / 56320 B (18427 B) | 31412 / 43008 B (11596 B) | 33881 / 49152 B (15271 B) |
| cmdapp | 96555 / 110592 B (14037 B) | 37900 / 56320 B (18420 B) | 31428 / 43008 B (11580 B) | 33877 / 49152 B (15275 B) |
| router | 117800 / 118784 B (984 B) | 45038 / 58368 B (13330 B) | 37057 / 45056 B (7999 B) | 40010 / 51200 B (11190 B) |
| router-dashboard | 234630 / 235520 B (890 B) | 94041 / 94208 B (167 B) | 77404 / 77824 B (420 B) | 82605 / 82944 B (339 B) |
| resource | 157459 / 157696 B (237 B) | 68688 / 69632 B (944 B) | 57813 / 58368 B (555 B) | 61514 / 62464 B (950 B) |

SHA-256 values identify every reviewed and final stream:

| app | reviewed raw / gzip / br / zstd SHA-256 | final raw / gzip / br / zstd SHA-256 |
| --- | --- | --- |
| counter | `1b4496b361964f115673be69193569e00c8f0783e4ffb0246192c956b5896457` / `ffa59e987bb5c96ad98f7dcaf3d3a593cdb239a2129c2c3cc8786ae49a48cf15` / `63d3868c49fca10129296cd0c948eec7100d9bc09355676a6326e93ce4ab6d58` / `0c7ebba2ac9804b1098d8bc3b2d54d252cb449f19e1a43ecc66f24aca55b1177` | `b2e0c0c44ba1a92a1aea4a5b24546e0babb01dca7819c839aece9a7c638fb03d` / `473585bde053c9c4902798dea12ce84e5552ed3fd4703d2cbbf7bb1d77483b43` / `ae5a5058606273bf2c93e591441f4588943521718f49520d0d996b4788922f90` / `9684c3b6b9c98a5386c2755d65356aab8103b468ad171d08fc154820c1249cba` |
| components | `287c9c495fbefbeb9bc7b11712d9baedef80b6d92c43ed5669be8602d8392c99` / `612a10723f323c7d496d0ce194ed1062cc9869dc3a5947f9cbc6840ac5a86944` / `a7c0ecacbd442fee675d520e4523b2aab11582cb10053ec635530845cad5bac5` / `190a851c4c070dac803b73498f82f25c8090584673473440217822ad9c76eed2` | `1784f025971927bc697f9d231185b00e5e94ebf7622207f03d42ea9ce947e3fe` / `8b0beb560773f20a7a0f2311ab564e2f93e620c00f95a1feac47223a3b78db01` / `cd59fc6b2f6108caf1aa0ab0826fded8271dab97e925b16cd57dc69a760d054f` / `87908961c33bae16290d5bef79fc7c928f4559570e290a725b4e1dd564dda42c` |
| todo | `132c288a9b0b418a378df200918e06500a8b8a3a29f84d68bf0311c624006d14` / `77dae4f05240c61db033ca6a239bdac956074cd6ac485351aa855215bf711108` / `b64e8508907a3a620e4bd19831162a20b8c06f5f3243b0cc73821e0bde796398` / `a4a1dad9e3da988c71825919692941ef50d283c11ddcbd7a87155d79859c0eb2` | `1a50a9b5ea1070aa2402d0ff2bccea5eaded4fef4a982f5fb7b76c139b002c1c` / `9aef5b284fe326d793f94628068d9d6ae9659641a7d4f02d2411d6c746ebf125` / `e3a620fd684cbf54c51696fac4003c360c3a41a5ec43607cdf23664ee1485b28` / `28ee84a5685ce253656ad78ceda6547a14bc03f70c404409197fdf970246479e` |
| dashboard | `03637d8cf568577acecc09012a13a37c74e948ef9238dec08221f209f7d2440d` / `ec2de8966b56c957fd4c689e49c0d1383ebe325cb68e6b22429021c7434fdef9` / `b3eaf0e24d5332de95d76c3d6f0bd88222cc47f097dcff59862d95f5a09a3d22` / `f5b0913a58e714432ef54b58b89494fdbadb22a4ef33621792e4cf25c56db320` | `1f8bae2c8b3a702626c5e021feff3ff58082e475331d385cf9fba652c9384779` / `21a4fde75886508b3fa151818a08325b6743e90aea47aee373ce00d4ea4bfaf8` / `f3bf6dbb038cc833269b31c097abd7c17189f66e7fc73aa2ecc4ebd1b9d7d837` / `7fd8208d1c336ebbdf2c5dd053e3b59ed4659a6d88110801e1337e4cab73d5ad` |
| context | `cf35b1eea5b6e4269326eea4948e21f6415643aa7265777bc84526b322e31fcd` / `7437eef051d0f521842c40cde485169e3dc820c9b6e9469135a2e9dd750f0861` / `397eb36a67c47dc1b93376d821551c880c22ee76e92e8bc464793177e9efabdb` / `b532e9fc3b4e6d078c8b476ec95ef8e2fd4fba89ce0e5cda247746f6097bee23` | `49d859d5e725438b1501f2356b609c77a950b06a0b12ec097d7185f6d75b9091` / `be5c89f0e06f778da15eb7305e4031be03b1f9331c87bb082375428e1ae45568` / `4c7e6ee5e3db933305e0dcd1ed5c578b165887752f444b24bcf14982b97dabfa` / `0ad2a4d85d30e7026870d58420ffc8157e64bafc32e54163c2b77261eec249b7` |
| virtualized | `01899b2c6ac85fe4421299921b3f960de8aa581698a9d5b9c6d8e7d5c105c92b` / `3cbb62348c76385ab69f6a823e6085ff29e849795a26802f3d77025d67bddb50` / `b7cfd6a391e4d273ba726af3c76c41c19ed0fd78c2ab22522335c3829e090592` / `304690e8391b4e63e68654283296cc554f886380344e1e470412496c0e9f0754` | `c581dfeda10e3f231b225dddbaf50d4d51bcde74ba9e9db8f0ffb579d1b0331d` / `eed6369e61fd88b8147e1a9bf918fed251d347e6532b5476bf1fae5efa27aa5d` / `02280681aea54fdc3fd69e15082a49989f543796b19dc27ab8a43fc731f8aa6c` / `89c245ea519728b1c4d88568638c80b1932225eff73fc2249505c79ce78ef2f7` |
| multipackage | `40616cedda7a2f644cf80c5b17b883bee56632be8e4842699c56d7651f2199ba` / `5e852872876ce689e2f243e2efd26d191dba45342f73a8b88005aa8c6c78a2c9` / `e40bc2c465e3f64db1a0e99f2138d5e3221aa7001ce927d8b1dd3e5aea71a946` / `51453d580001c593059068056207d9c4fbdb14f36867861f1f3f3a0f7f036b22` | `ce1131d527e2af8ad557696f5ac0c29c0fc4108a95158848d5e5fb3123a9001f` / `ddf75ad44bd78b1c422868f0d0780fc6b125cbf1961a546727aa8c07ccef7b25` / `ae59ce973fd63fda26ef0464500cb424586b9ff31b08c53ca0eb253cd1af3ff3` / `c9fd085f0537ac86015a77b527f8c368794a0b2377dc0e8915eac09d3fcd24d7` |
| cmdapp | `4ee342784b97bb2e50cf1830959bfa45ba8304155c2c384ececce22f11181eb7` / `756a548ef19365ecf27605d1041a327a8be26d77324f4463e96d44259154df35` / `61fff06eeaa56f852a9e082e85b7acf5ac49fa51839b1a92c0d88332de9e13e2` / `981549f73bfcfa587a3a050e653f85e69922f73d3b6826fad3fa62324d1dba7d` | `8b389fe2a06d0a2ae4aa2543c0c5b874ad40f3026a33d98a97469f7ef9e0385c` / `7adbf071d48e61375cb0e29766b31016d86b9425f2a678708119ef048698c239` / `6c52bb7f63563f0f98b696c0c31db8cb7eb68591b92aae07c42cf9289c4a3589` / `e87f96f306fe32581d3531db004aa2afb6a6dccc87afe5b93ca8861e41556d89` |
| router | `df159357d037b5242b0bd297aa3b8819080d3b14e6ec3926fed97c7d51c3ce99` / `6014712186e295844c1b3bba44be15a7a6b9e1ba385fbec6f26383cd31d09183` / `2a2742d8b30a38d9db438e26ec64569ed995f3badd72c9a02271a12fc3cf7315` / `43e7f3e522df944f16254e913260dbc8699c1c310303930dd0e917d20ca82758` | `95c88f56b1fa40054a1008d4813ad5455710076742a06fb26be801765076b293` / `df4cb1e4323fed7ce168c4860a7623addfcd66178df42603a19d2603d9d2b25a` / `466c10552774f166a67a409e43381cca5c2a2661b3534a4ec9774b028eb7dd1d` / `a29fa10c1cb630d1e29a988a570e07ef6957aa6fd1d941659c909720152a14c4` |
| router-dashboard | `8cf8e96b377c9108404f36b8fe2bfa9216df428fb2ac34037262656902496892` / `090c912909f9ad6ce90c4b619c69b30c72b24b8bd2cea5ae26e9d1d57a26179a` / `25cb0da315951534908265225993d1346ffefa09355f2785c4cd4d1390144be1` / `3c3529f718bcc2992438a335d3bdbef49c2437ce1d72093d6c15c4963239848c` | `ca32baf21d6575699d0de00d2e3af46587794393509c04a053d650a3b56dbcef` / `da7796caf4c12a364c28440e86070aaaeea4f0c979aff6ec5fda587d87502506` / `10ff8380d1863414099cce7ca4922a621318d80eaa0323321f5f5fd405dbd8de` / `aa47210fd11563e2378b024ced6f7ed692a5db070816ee7b17c5599be819de9d` |
| resource | `5827b43710546eb152e8430b22579053064ab4f9c2395b1122212f740e0e405a` / `13d18c2c42094170328c6dabe61db9379deee8b44695d45c938acbead12912a8` / `2e9de7a76a7d403d99e2df9f24152e42f13467a7bccb2bb67535961b06788db5` / `e7758fb4623f32d04facd20148c4cdb2e07e02af1323c8384f66ad94a6b73720` | `422d330c2d804f65c16ca67dff6d0d3feba81d1d5d55e0b48d0b766a7d04d301` / `66c705aa25e02c39a6e5f50f2b0c73f0e06082e4594109a0b9c79ecbd2e476e7` / `5599eb5bc7ab488444e99257d62050c32d89ad8d26d87ca9ae2b5df79544060f` / `b1d9c57aead2dbaf0eb2e440b6c059c14a3a9d406ad0cf94bf40f8c0294b023c` |

The old ceilings failed only at `router` raw (`117800 / 117760 B`),
`router-dashboard` raw (`234630 / 234496 B`), and `resource` Zstandard
(`61514 / 61440 B`). Their new ceilings are `118784 B`, `235520 B`, and
`62464 B`, respectively, each exactly `1024 B` higher and leaving `984 B`,
`890 B`, and `950 B` headroom. All 44 final cells and all ratio limits pass.
No other ceiling, ratio, compression command, workflow, or matrix membership
changed. This is an accepted root-ownership correctness cost; the private
repeated-Mount fixture remains outside the size matrix.

## Targeted ErrorBoundary Correctness Rebaseline — 2026-07-28

This targeted rebaseline covers complete mounted-subtree teardown ownership
during protected ErrorBoundary reconciliation. The frozen main base is
`15a0b8fe4f5d3f0da79b57acc10e1fab4e6cbac5`; the PR head before this
correction is `ee68d49296586a42d142b9b4031fcb173f127b2d`. The runtime correction
is `a3463d9d88837a3adb46d30f1d03e1e8c1947d1c`, and the browser evidence is
`481d8ea5945caec08ac57668af49e3dc456e2762`.

Measurements use Go `1.22.12` and TinyGo `0.41.1` on Linux amd64. Compression
uses the unchanged budget-script commands. Head-to-final deltas compare
artifacts with the same `bundle.wasm` basename.

| app/format | PR head size | final size | delta | old budget | new budget | final headroom |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| router-dashboard raw | 230316 B | 232586 B | +2270 B | 230400 B | 233472 B | 886 B |
| router-dashboard gzip | 92577 B | 93271 B | +694 B | 94208 B | 94208 B | 937 B |
| router-dashboard br | 76183 B | 76703 B | +520 B | 77824 B | 77824 B | 1121 B |
| router-dashboard zstd | 81281 B | 81813 B | +532 B | 81920 B | 81920 B | 107 B |
| resource raw | 153200 B | 155484 B | +2284 B | 153600 B | 156672 B | 1188 B |
| resource gzip | 67158 B | 67897 B | +739 B | 67584 B | 68608 B | 711 B |
| resource br | 56578 B | 57182 B | +604 B | 57344 B | 57344 B | 162 B |
| resource zstd | 59968 B | 60748 B | +780 B | 61440 B | 61440 B | 692 B |

The nine non-ErrorBoundary applications (`counter`, `components`, `todo`,
`dashboard`, `context`, `virtualized`, `multipackage`, `cmdapp`, and `router`)
remain byte-identical at the raw WASM SHA-256 boundary; their compressed sizes
are also unchanged. The final compression ratios remain within the existing
limits: gzip `52.00%`, Brotli `38.00%`, and Zstandard `46.00%`.

At that historical stage, the earlier recommendation to keep budgets unchanged
was superseded only for this accepted ErrorBoundary correctness guarantee. Raw
budgets increased by 3 KiB for `router-dashboard` and `resource`. Resource gzip
increased by 1 KiB under the aligned compressed-budget rule; the other
compressed budgets remained unchanged because their final artifacts fit. No
workflow or ratio limit changes, no unrelated application budget changes, and
no general runtime headroom were authorized by this rebaseline.

## Historical Toolchain Matrix

This matrix records the original audit environment. The supported baseline
migration above supersedes it as current CI guidance.

| environment | Go version | TinyGo version | goxc version | reproduction method | status |
| --- | --- | --- | --- | --- | --- |
| local current toolchain | `go1.24.4 linux/amd64` | `0.41.1`, using Go `1.24.4` | `devel` in temp clone; installed host `goxc` reported `v0.2.0-preview.3` | clean temp clone, full workflow package sequence | failed dashboard raw by `829 B` |
| CI-like container | `go1.22.12 linux/amd64` | `0.41.1`, using Go `1.22.12` | `devel` in container clone | Docker `golang:1.22-bookworm`, TinyGo `0.41.1`, full workflow package sequence | passed |

Local Go `1.22.x` binaries were not installed. Docker was available, so the
CI-like reproduction used a self-contained container clone. Host bind mounts
from `/tmp` were not visible inside the Docker daemon namespace, so the
container cloned `https://github.com/graybuton/goframe.git` directly and report
files were copied out with `docker cp`.

## Size Results

### Local Current Toolchain

Toolchain: Go `1.24.4`, TinyGo `0.41.1` using Go `1.24.4`.

| app | selected wasm path | raw size | raw budget | raw delta | gzip | br | zstd | overall |
| --- | --- | ---: | ---: | ---: | --- | --- | --- | --- |
| counter | `examples/counter/.goframe/package/standalone/assets/bundle.26d7a4ef.wasm` | 84710 B | 97280 B | -12570 B | pass | pass | pass | pass |
| components | `examples/components/.goframe/package/standalone/assets/bundle.c61bdba9.wasm` | 90357 B | 107520 B | -17163 B | pass | pass | pass | pass |
| todo | `examples/todo/.goframe/package/standalone/assets/bundle.a278fa54.wasm` | 118568 B | 122880 B | -4312 B | pass | pass | pass | pass |
| dashboard | `examples/dashboard/.goframe/package/standalone/assets/bundle.66835bbd.wasm` | 169789 B | 168960 B | +829 B | pass | pass | pass | fail |
| context | `examples/context/.goframe/package/standalone/assets/bundle.d0e2edab.wasm` | 116505 B | 116736 B | -231 B | pass | pass | pass | pass |
| virtualized | `examples/virtualized/.goframe/package/standalone/assets/bundle.653e65fe.wasm` | 124306 B | 124928 B | -622 B | pass | pass | pass | pass |
| multipackage | `examples/multipackage/.goframe/package/standalone/assets/bundle.146e4abe.wasm` | 95514 B | 110592 B | -15078 B | pass | pass | pass | pass |
| cmdapp | `examples/cmdapp/.goframe/package/standalone/assets/bundle.14826b8b.wasm` | 95540 B | 110592 B | -15052 B | pass | pass | pass | pass |
| router | `examples/router/.goframe/package/standalone/assets/bundle.22ebe49b.wasm` | 115879 B | 116736 B | -857 B | pass | pass | pass | pass |
| router-dashboard | `examples/router-dashboard/.goframe/package/standalone/assets/bundle.2b9c87c5.wasm` | 227447 B | 230400 B | -2953 B | pass | pass | pass | pass |
| resource | `examples/resource/.goframe/package/standalone/assets/bundle.c9c1cb5d.wasm` | 150210 B | 153600 B | -3390 B | pass | pass | pass | pass |

### CI-Like Go 1.22 Toolchain

Toolchain: Go `1.22.12`, TinyGo `0.41.1` using Go `1.22.12`.

| app | selected wasm path | raw size | raw budget | raw delta | gzip | br | zstd | overall |
| --- | --- | ---: | ---: | ---: | --- | --- | --- | --- |
| counter | `examples/counter/.goframe/package/standalone/assets/bundle.313897da.wasm` | 83867 B | 97280 B | -13413 B | pass | pass | pass | pass |
| components | `examples/components/.goframe/package/standalone/assets/bundle.0e720a2b.wasm` | 89507 B | 107520 B | -18013 B | pass | pass | pass | pass |
| todo | `examples/todo/.goframe/package/standalone/assets/bundle.97a210dc.wasm` | 117700 B | 122880 B | -5180 B | pass | pass | pass | pass |
| dashboard | `examples/dashboard/.goframe/package/standalone/assets/bundle.eb4676f0.wasm` | 168907 B | 168960 B | -53 B | pass | pass | pass | pass |
| context | `examples/context/.goframe/package/standalone/assets/bundle.4b92c0e0.wasm` | 115663 B | 116736 B | -1073 B | pass | pass | pass | pass |
| virtualized | `examples/virtualized/.goframe/package/standalone/assets/bundle.431d82c8.wasm` | 123454 B | 124928 B | -1474 B | pass | pass | pass | pass |
| multipackage | `examples/multipackage/.goframe/package/standalone/assets/bundle.b81c6cab.wasm` | 94671 B | 110592 B | -15921 B | pass | pass | pass | pass |
| cmdapp | `examples/cmdapp/.goframe/package/standalone/assets/bundle.dcec4282.wasm` | 94689 B | 110592 B | -15903 B | pass | pass | pass | pass |
| router | `examples/router/.goframe/package/standalone/assets/bundle.324f2b8d.wasm` | 114856 B | 116736 B | -1880 B | pass | pass | pass | pass |
| router-dashboard | `examples/router-dashboard/.goframe/package/standalone/assets/bundle.529f28a9.wasm` | 226171 B | 230400 B | -4229 B | pass | pass | pass | pass |
| resource | `examples/resource/.goframe/package/standalone/assets/bundle.4b41b5b6.wasm` | 148985 B | 153600 B | -4615 B | pass | pass | pass | pass |

## Dashboard Findings

- Current local dashboard raw size under Go `1.24.4` is `169789 B`, which is
  `829 B` over the raw budget.
- A clean temp clone under the same local Go `1.24.4` and TinyGo `0.41.1`
  reproduced the same `169789 B` dashboard raw size. This rules out stale
  ignored artifacts as the primary cause of the local failure.
- The CI-like Go `1.22.12` reproduction produced `168907 B`, which is `53 B`
  under the dashboard raw budget.
- The most likely explanation is Go-version-sensitive TinyGo output, not local
  artifact drift.
- The remaining dashboard raw headroom under the CI-like toolchain is too small
  for runtime experiments. A small production helper can plausibly consume more
  than `53 B` even when it is well scoped.

## Runtime Size Surface

The dashboard example uses `gf.VirtualTable`, state, effects, prop
normalization, component rendering, focus preservation, and event handlers. It
does not appear to use router, resource, fetch, or context APIs directly.

### Event Wrapper Path

- Files: `pkg/goframe/render_js.go`, `pkg/goframe/event.go`
- Why it may affect dashboard: dashboard has click, input, change, and scroll
  handlers. `eventHandler` currently constructs `Event`, `InputEvent`, and
  `ScrollEvent` wrappers before switching on the callback type.
- Risk: medium. Event callback behavior is user-facing and browser-smoke
  sensitive.
- Expected size impact: medium estimate. This is unmeasured.
- Follow-up recommendation: measure a type-switch-first event path that creates
  only the wrapper needed by the actual callback type.

### Virtual Table Helpers

- Files: `pkg/goframe/virtual.go`
- Why it may affect dashboard: dashboard renders its table through
  `gf.VirtualTable`.
- Risk: medium. Virtualization behavior is visible and already covered by tests
  and browser smoke.
- Expected size impact: medium estimate if string/style construction or
  callback recovery paths can be simplified. This is unmeasured.
- Follow-up recommendation: audit `VirtualTable` style/key helpers and render
  callback recovery paths, keeping fixed-row semantics unchanged.

### Runtime Error Recovery Metadata

- Files: `pkg/goframe/component.go`, `pkg/goframe/effects.go`,
  `pkg/goframe/render_js.go`, `pkg/goframe/virtual.go`,
  `pkg/goframe/error_boundary.go`, `pkg/goframe/errors.go`
- Why it may affect dashboard: component render, memo, effect, event, and
  virtual render callbacks include recovered-panic reporting paths and
  operation strings.
- Risk: medium to high. Error reporting and Error Boundary behavior are part of
  the runtime contract.
- Expected size impact: medium estimate. This is unmeasured.
- Follow-up recommendation: measure whether repeated operation strings or
  per-callback recovery wrappers can be compacted without changing
  `ErrorInfo`, Error Boundary, or panic containment behavior.

### Prop Normalization and Primitive Conversion

- Files: `pkg/goframe/props.go`
- Why it may affect dashboard: dashboard emits many DOM props and events.
  `splitProps`, `eventNameForProp`, `normalizeAttributeName`, and `ToString`
  are hot and linked.
- Risk: medium. Previous PRs already characterized prop behavior and
  allocations; behavior must remain exactly compatible.
- Expected size impact: low to medium estimate. This is unmeasured.
- Follow-up recommendation: measure ASCII-specialized normalization and a
  narrower conversion path only if existing tests remain unchanged.

### Focus Preservation

- Files: `pkg/goframe/mount_js.go`
- Why it may affect dashboard: every dirty flush captures and restores focus
  around DOM patches.
- Risk: high. This is visible UX behavior for focused inputs and selection.
- Expected size impact: unknown.
- Follow-up recommendation: do not optimize first. Only revisit with browser
  smoke coverage for focused input and selection behavior.

### Router Query Helpers

- Files: `pkg/goframe/router.go`, `pkg/goframe/router_js.go`
- Why it may affect dashboard: likely not linked into the plain dashboard
  example, but relevant to `router` and `router-dashboard` budgets.
- Risk: medium. Query encoding and matching behavior are public API.
- Expected size impact: low for dashboard, unknown for router examples.
- Follow-up recommendation: keep separate from dashboard headroom work.

### Resource and Fetch Helpers

- Files: `pkg/goframe/resource.go`, `pkg/goframe/fetch_js.go`
- Why it may affect dashboard: likely not linked into dashboard, but relevant
  to `resource` and `server-backed` evidence.
- Risk: medium. Cleanup, stale completion, and abort behavior are behavioral
  contracts.
- Expected size impact: low for dashboard, unknown for resource examples.
- Follow-up recommendation: do not use resource/fetch cleanup to recover
  dashboard headroom.

### Context Selector Topology

- Files: `pkg/goframe/context.go`
- Why it may affect dashboard: dashboard does not appear to use context.
- Risk: high. Provider topology and selector invalidation behavior are subtle.
- Expected size impact: low for dashboard, unknown for context example.
- Follow-up recommendation: not a first dashboard-size candidate.

## Cleanup Candidates

| priority | candidate | files | expected size impact | risk | proposed PR title | validation |
| ---: | --- | --- | --- | --- | --- | --- |
| 1 | Construct only the event wrapper required by the callback type | `pkg/goframe/render_js.go`, `pkg/goframe/event.go` if needed | medium estimate | medium | `perf(wasm): slim event wrapper construction` | `go test ./pkg/goframe`; `go test ./...`; `go vet ./...`; `scripts/size-budget.sh`; `scripts/browser-smoke.sh` |
| 2 | Slim `VirtualTable` helper/style construction without changing fixed-row behavior | `pkg/goframe/virtual.go` | medium estimate | medium | `perf(wasm): reduce virtual table runtime size` | `go test ./pkg/goframe -run 'TestVirtual'`; `go test ./...`; `scripts/size-budget.sh`; `scripts/browser-smoke.sh` |
| 3 | Compact runtime error operation metadata while preserving `ErrorInfo` semantics | `pkg/goframe/component.go`, `pkg/goframe/effects.go`, `pkg/goframe/render_js.go`, `pkg/goframe/virtual.go`, `pkg/goframe/errors.go` | medium estimate | medium/high | `perf(wasm): compact runtime error reporting paths` | error-boundary/error-handling focused tests; `go test ./...`; `scripts/browser-smoke.sh`; `scripts/size-budget.sh` |
| 4 | Measure ASCII-specialized prop event/attribute normalization | `pkg/goframe/props.go` | low/medium estimate | medium | `perf(wasm): simplify prop normalization` | splitProps tests and benchmarks; `go test ./...`; `scripts/size-budget.sh`; `scripts/browser-smoke.sh` |
| 5 | Router query helper size pass for router examples only | `pkg/goframe/router.go` | low for dashboard, unknown elsewhere | medium | `perf(router): audit query helper size` | router tests; router browser smoke; `scripts/size-budget.sh` |

The first follow-up should target dashboard-linked code and should measure size
after each production change. Test-only and benchmark-only changes do not help
the dashboard WASM size.

## Historical Recommendation (Superseded 2026-07-30)

The following recommendation records the earlier Go `1.22.12` source-of-truth
decision. The evidence-backed supported toolchain migration above is the only
supersession of its toolchain source-of-truth guidance; it does not alter the
historical runtime cleanup observations.

- Do not update the size workflow to Go `1.24` yet. The local Go `1.24.4`
  reproduction fails the dashboard raw budget by `829 B`.
- Keep budgets unchanged for now. The CI-like Go `1.22.12` reproduction passes,
  but dashboard has only `53 B` of raw headroom.
- Pause LIS and other runtime feature work that adds production code until at
  least 1-4 KB of dashboard raw headroom is recovered under the CI-like
  toolchain.
- Use Go `1.22.x` plus TinyGo `0.41.1` for local release-size decisions, or
  treat the GitHub WASM Size workflow as authoritative when local Go differs.
- Recommended next PR: `perf(wasm): slim event wrapper construction`.

## Appendix

### Commands Run

Base and validation:

```sh
git fetch origin --tags --prune
git switch main
git pull --ff-only origin main
git status --short
git rev-parse HEAD
git rev-parse origin/main
git rev-parse v0.2.0-preview.3^{commit}
git switch -c audit/wasm-runtime-size-headroom
go test ./pkg/goframe
go test ./...
go vet ./...
git diff --check
```

Source-of-truth inspection:

```sh
sed -n '1,240p' .github/workflows/ci-wasm-size.yml
sed -n '1,260p' scripts/size-budget.sh
```

Local toolchain inspection:

```sh
go version
tinygo version
which go
which tinygo
go env GOOS GOARCH GOVERSION
goxc version || true
which goxc || true
command -v go1.22 || true
command -v go1.22.12 || true
ls "$HOME/sdk" 2>/dev/null || true
command -v docker || true
command -v podman || true
```

Current-toolchain reproduction:

```sh
git clone --no-local /home/jin-wu/solutions/repos/goframe "$tmpdir/goframe-size-go-current"
git checkout 587c06d1345415960cd9100d42fa81df360fc1a0
GOBIN="$tmpbin" go install ./cmd/goxc
goxc generate ./examples/<app>
goxc package ./examples/<app> --compiler=tinygo
goxc package ./examples/<app> --compiler=tinygo --asset-hash --preload --compress=gzip,br
scripts/size-budget.sh | tee /tmp/goframe-size-current-report.txt
```

CI-like Go `1.22` reproduction:

```sh
docker run --name "$name" golang:1.22-bookworm bash -lc '...'
docker cp "$name:/reports/goframe-size-go122-report.txt" /tmp/goframe-size-go122-report.txt
docker cp "$name:/reports/goframe-size-go122-paths.txt" /tmp/goframe-size-go122-paths.txt
docker rm "$name"
```

Runtime surface inspection:

```sh
rg -n "panic\(|fmt\.|reflect\.|recover\(|ErrorInfo|report|debug|runtimeComponentName|strings\.|strconv\.|syscall/js" pkg/goframe
rg -n "func .*\(" pkg/goframe
rg -n "go:build|goframe_debug" pkg/goframe
rg -n "goframe|UseResource|FetchText|Router|Virtual|UseContext|UseEffect|UseState|On[A-Z]|on[A-Z]|Component|ErrorBoundary" examples/dashboard
```

Optional artifact tools checked:

```sh
command -v wasm-objdump || true
command -v wasm-tools || true
command -v twiggy || true
command -v llvm-size || true
command -v wasm2wat || true
```

None were available locally during this audit.

### Temporary Reports

- `/tmp/goframe-size-current-paths.txt`
- `/tmp/goframe-size-current-report.txt`
- `/tmp/goframe-size-go122-paths.txt`
- `/tmp/goframe-size-go122-report.txt`

### Important Output Excerpts

Local current toolchain:

```text
go version go1.24.4 linux/amd64
tinygo version 0.41.1 linux/amd64 (using go version go1.24.4 and LLVM version 20.1.1)
dashboard    raw     169789 B /   168960 B
dashboard    raw over budget by 829 B
```

CI-like Go `1.22` toolchain:

```text
go version go1.22.12 linux/amd64
tinygo version 0.41.1 linux/amd64 (using go version go1.22.12 and LLVM version 20.1.1)
dashboard    raw     168907 B /   168960 B
```

### Known Limitations

- Cleanup candidate size impacts are estimates, not measured results.
- Optional WASM symbol-inspection tools were not installed, so this audit did
  not attribute bytes to individual functions.
- The CI-like reproduction used Docker and cloned from GitHub inside the
  container because host `/tmp` bind mounts were not visible inside the Docker
  daemon namespace.
- The original audit did not change runtime code, size budgets, workflows,
  examples, or generated artifacts. The later supported-toolchain migration
  changes only workflow versions and the three measured budget cells documented
  above.
