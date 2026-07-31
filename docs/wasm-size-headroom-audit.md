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
| router | 117760 B | 45056 B | 58368 B | 51200 B |
| router-dashboard | 234496 B | 77824 B | 94208 B | 82944 B |
| resource | 157696 B | 58368 B | 69632 B | 61440 B |

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
