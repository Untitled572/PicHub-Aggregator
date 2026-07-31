# Tag 匹配规则设计

## 规则

**核心条件：`S ∩ Q ≠ ∅`**（源和用户选择至少有一个共同 tag）

**例外：Q 包含任一 `exclusive=true` 的 tag 时，`__uncategorized__` 源不通过**

## 标签定义

| Tag | System | Exclusive |
|---|---|---|---|
| horizontal | true | false |
| vertical | true | false |
| r18 | false | true |
| nsfw | false | true |
| anime | false | false |
| landscape | false | false |
| photography | false | false |

---

## 大表

| # | 用户选中 Q | 源 tag S | 交集 | exclusive 保护 ?uncat? | 结果 |
|---|---|---|---|---|---|
| 1 | {r18} | {r18} | ✅ r18 | — | 是 |
| 2 | {r18} | {anime} | ❌ | — | 否 |
| 3 | {r18} | {r18, anime} | ✅ r18 | — | 是 |
| 4 | {r18} | {\_\_uncategorized\_\_} | ❌ | ✅ r18.exclusive=true → 排除 | 否 |
| 5 | {nsfw} | {nsfw} | ✅ nsfw | — | 是 |
| 6 | {nsfw} | {anime} | ❌ | — | 否 |
| 7 | {nsfw} | {nsfw, anime} | ✅ nsfw | — | 是 |
| 8 | {nsfw} | {\_\_uncategorized\_\_} | ❌ | ✅ nsfw.exclusive=true → 排除 | 否 |
| 9 | {r18, nsfw} | {r18} | ✅ r18 | — | 是 |
| 10 | {r18, nsfw} | {nsfw} | ✅ nsfw | — | 是 |
| 11 | {r18, nsfw} | {anime} | ❌ | — | 否 |
| 12 | {r18, nsfw} | {\_\_uncategorized\_\_} | ❌ | ✅ 两个 exclusive → 排除 | 否 |
| 13 | {anime} | {anime} | ✅ anime | — | 是 |
| 14 | {anime} | {r18} | ❌ | — | 否 |
| 15 | {anime} | {nsfw} | ❌ | — | 否 |
| 16 | {anime} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | N |
| 17 | {horizontal} | {horizontal} | ✅ horizontal | — | 是 |
| 18 | {horizontal} | {vertical} | ❌ | — | 否 |
| 19 | {horizontal} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | N |
| 20 | {vertical} | {vertical} | ✅ vertical | — | 是 |
| 21 | {vertical} | {horizontal} | ❌ | — | 否 |
| 22 | {vertical} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | N |
| 23 | {r18, anime} | {r18} | ✅ r18 | — | 是 |
| 24 | {r18, anime} | {anime} | ✅ anime | — | 是 |
| 25 | {r18, anime} | {r18, anime} | ✅ both | — | 是 |
| 26 | {r18, anime} | {horizontal} | ❌ | — | 否 |
| 27 | {r18, anime} | {r18, horizontal} | ✅ r18 | — | 是 |
| 28 | {r18, anime} | {nsfw} | ❌ | — | 否 |
| 29 | {r18, anime} | {\_\_uncategorized\_\_} | ❌ | ✅ r18.exclusive=true → 排除 | 否 |
| 30 | {nsfw, anime} | {nsfw} | ✅ nsfw | — | 是 |
| 31 | {nsfw, anime} | {anime} | ✅ anime | — | 是 |
| 32 | {nsfw, anime} | {r18} | ❌ | — | 否 |
| 33 | {nsfw, anime} | {\_\_uncategorized\_\_} | ❌ | ✅ nsfw.exclusive=true → 排除 | 否 |
| 34 | {r18, nsfw, anime} | {r18} | ✅ r18 | — | 是 |
| 35 | {r18, nsfw, anime} | {anime} | ✅ anime | — | 是 |
| 36 | {r18, nsfw, anime} | {horizontal} | ❌ | — | 否 |
| 37 | {r18, nsfw, anime} | {\_\_uncategorized\_\_} | ❌ | ✅ r18/nsfw exclusive → 排除 | 否 |
| 38 | {anime, landscape} | {anime} | ✅ anime | — | 是 |
| 39 | {anime, landscape} | {landscape} | ✅ landscape | — | 是 |
| 40 | {anime, landscape} | {r18} | ❌ | — | 否 |
| 41 | {anime, landscape} | {nsfw} | ❌ | — | 否 |
| 42 | {anime, landscape} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | 是 |
| 43 | {r18, nsfw, landscape} | {r18} | ✅ r18 | — | 是 |
| 44 | {r18, nsfw, landscape} | {nsfw} | ✅ nsfw | — | 是 |
| 45 | {r18, nsfw, landscape} | {landscape} | ✅ landscape | — | 是 |
| 46 | {r18, nsfw, landscape} | {anime} | ❌ | — | 否 |
| 47 | {r18, nsfw, landscape} | {r18, anime} | ✅ r18 | — | 是 |
| 48 | {r18, nsfw, landscape} | {\_\_uncategorized\_\_} | ❌ | ✅ r18/nsfw exclusive → 排除 | 否 |
| 49 | {horizontal, vertical} | {horizontal} | ✅ horizontal | — | 是 |
| 50 | {horizontal, vertical} | {vertical} | ✅ vertical | — | 是 |
| 51 | {horizontal, vertical} | {r18} | ❌ | — | 否 |
| 52 | {horizontal, vertical} | {nsfw} | ❌ | — | 否 |
| 53 | {horizontal, vertical} | {r18, horizontal} | ✅ horizontal | — | 是 |
| 54 | {horizontal, vertical} | {nsfw, vertical} | ✅ vertical | — | 是 |
| 55 | {horizontal, vertical} | {r18, nsfw} | ❌ | — | 否 |
| 56 | {horizontal, vertical} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | 是 |
| 57 | {r18, horizontal} | {r18} | ✅ r18 | — | 是 |
| 58 | {r18, horizontal} | {horizontal} | ✅ horizontal | — | 是 |
| 59 | {r18, horizontal} | {r18, horizontal} | ✅ both | — | 是 |
| 60 | {r18, horizontal} | {vertical} | ❌ | — | 否 |
| 61 | {r18, horizontal} | {nsfw} | ❌ | — | 否 |
| 62 | {r18, horizontal} | {\_\_uncategorized\_\_} | ❌ | ✅ r18.exclusive=true → 排除 | 否 |
| 63 | {nsfw, vertical} | {nsfw} | ✅ nsfw | — | 是 |
| 64 | {nsfw, vertical} | {vertical} | ✅ vertical | — | 是 |
| 65 | {nsfw, vertical} | {r18} | ❌ | — | 否 |
| 66 | {nsfw, vertical} | {\_\_uncategorized\_\_} | ❌ | ✅ nsfw.exclusive=true → 排除 | 否 |
| 67 | {r18, nsfw, horizontal, vertical} | {r18, horizontal} | ✅ r18 | — | 是 |
| 68 | {r18, nsfw, horizontal, vertical} | {nsfw, vertical} | ✅ nsfw/vertical | — | 是 |
| 69 | {r18, nsfw, horizontal, vertical} | {anime} | ❌ | — | 否 |
| 70 | {r18, nsfw, horizontal, vertical} | {\_\_uncategorized\_\_} | ❌ | ✅ r18/nsfw exclusive → 排除 | 否 |
| 71 | {photography} | {photography} | ✅ photography | — | 是 |
| 72 | {photography} | {r18} | ❌ | — | 否 |
| 73 | {photography} | {nsfw} | ❌ | — | 否 |
| 74 | {photography} | {\_\_uncategorized\_\_} | ❌ | |Q|=1 严格匹配 | 否 |
| 75 | {r18, nsfw, anime, landscape, photography} | {r18} | ✅ r18 | — | 是 |
| 76 | {r18, nsfw, anime, landscape, photography} | {nsfw} | ✅ nsfw | — | 是 |
| 77 | {r18, nsfw, anime, landscape, photography} | {anime, landscape} | ✅ both | — | 是 |
| 78 | {r18, nsfw, anime, landscape, photography} | {r18, landscape} | ✅ r18 | — | 是 |
| 79 | {r18, nsfw, anime, landscape, photography} | {horizontal} | ❌ | — | 否 |
| 80 | {r18, nsfw, anime, landscape, photography} | {\_\_uncategorized\_\_} | ❌ | ✅ r18/nsfw exclusive → 排除 | 否 |
| 81 | {anime, photography} | {anime} | ✅ anime | — | 是 |
| 82 | {anime, photography} | {photography} | ✅ photography | — | 是 |
| 83 | {anime, photography} | {r18} | ❌ | — | 否 |
| 84 | {anime, photography} | {nsfw, anime} | ✅ anime | — | 是 |
| 85 | {anime, photography} | {r18, nsfw} | ❌ | — | 否 |
| 86 | {anime, photography} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | 是 |
| 87 | {r18, nsfw, horizontal} | {anime, landscape, photography} | ❌ | — | 否 |
| 88 | {}（无选中） | {r18} | N/A | — | 是 |
| 89 | {} | {nsfw} | N/A | — | 是 |
| 90 | {} | {anime, landscape} | N/A | — | 是 |
| 91 | {} | {\_\_uncategorized\_\_} | N/A | — | 是 |
