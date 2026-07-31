# Tag 匹配规则设计

## 规则

**核心条件：`S ∩ Q ≠ ∅`**（源和用户选择至少有一个共同 tag）

**例外：Q 包含任一 `exclusive=true` 的 tag 时，`__uncategorized__` 源不通过**

## 标签定义

| Tag | System | Exclusive |
|---|---|---|---|
| horizontal | true | false |
| vertical | true | false |
| square | false | true |
| nsfw | false | true |
| anime | false | false |
| landscape | false | false |
| photography | false | false |

---

## 大表

| # | 用户选中 Q | 源 tag S | 交集 | exclusive 保护 ?uncat? | 结果 |
|---|---|---|---|---|---|
| 1 | {square} | {square} | ✅ square | — | 是 |
| 2 | {square} | {anime} | ❌ | — | 否 |
| 3 | {square} | {square, anime} | ✅ square | — | 是 |
| 4 | {square} | {\_\_uncategorized\_\_} | ❌ | ✅ square.exclusive=true → 排除 | 否 |
| 5 | {nsfw} | {nsfw} | ✅ nsfw | — | 是 |
| 6 | {nsfw} | {anime} | ❌ | — | 否 |
| 7 | {nsfw} | {nsfw, anime} | ✅ nsfw | — | 是 |
| 8 | {nsfw} | {\_\_uncategorized\_\_} | ❌ | ✅ nsfw.exclusive=true → 排除 | 否 |
| 9 | {square, nsfw} | {square} | ✅ square | — | 是 |
| 10 | {square, nsfw} | {nsfw} | ✅ nsfw | — | 是 |
| 11 | {square, nsfw} | {anime} | ❌ | — | 否 |
| 12 | {square, nsfw} | {\_\_uncategorized\_\_} | ❌ | ✅ 两个 exclusive → 排除 | 否 |
| 13 | {anime} | {anime} | ✅ anime | — | 是 |
| 14 | {anime} | {square} | ❌ | — | 否 |
| 15 | {anime} | {nsfw} | ❌ | — | 否 |
| 16 | {anime} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | N |
| 17 | {horizontal} | {horizontal} | ✅ horizontal | — | 是 |
| 18 | {horizontal} | {vertical} | ❌ | — | 否 |
| 19 | {horizontal} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | N |
| 20 | {vertical} | {vertical} | ✅ vertical | — | 是 |
| 21 | {vertical} | {horizontal} | ❌ | — | 否 |
| 22 | {vertical} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | N |
| 23 | {square, anime} | {square} | ✅ square | — | 是 |
| 24 | {square, anime} | {anime} | ✅ anime | — | 是 |
| 25 | {square, anime} | {square, anime} | ✅ both | — | 是 |
| 26 | {square, anime} | {horizontal} | ❌ | — | 否 |
| 27 | {square, anime} | {square, horizontal} | ✅ square | — | 是 |
| 28 | {square, anime} | {nsfw} | ❌ | — | 否 |
| 29 | {square, anime} | {\_\_uncategorized\_\_} | ❌ | ✅ square.exclusive=true → 排除 | 否 |
| 30 | {nsfw, anime} | {nsfw} | ✅ nsfw | — | 是 |
| 31 | {nsfw, anime} | {anime} | ✅ anime | — | 是 |
| 32 | {nsfw, anime} | {square} | ❌ | — | 否 |
| 33 | {nsfw, anime} | {\_\_uncategorized\_\_} | ❌ | ✅ nsfw.exclusive=true → 排除 | 否 |
| 34 | {square, nsfw, anime} | {square} | ✅ square | — | 是 |
| 35 | {square, nsfw, anime} | {anime} | ✅ anime | — | 是 |
| 36 | {square, nsfw, anime} | {horizontal} | ❌ | — | 否 |
| 37 | {square, nsfw, anime} | {\_\_uncategorized\_\_} | ❌ | ✅ square/nsfw exclusive → 排除 | 否 |
| 38 | {anime, landscape} | {anime} | ✅ anime | — | 是 |
| 39 | {anime, landscape} | {landscape} | ✅ landscape | — | 是 |
| 40 | {anime, landscape} | {square} | ❌ | — | 否 |
| 41 | {anime, landscape} | {nsfw} | ❌ | — | 否 |
| 42 | {anime, landscape} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | 是 |
| 43 | {square, nsfw, landscape} | {square} | ✅ square | — | 是 |
| 44 | {square, nsfw, landscape} | {nsfw} | ✅ nsfw | — | 是 |
| 45 | {square, nsfw, landscape} | {landscape} | ✅ landscape | — | 是 |
| 46 | {square, nsfw, landscape} | {anime} | ❌ | — | 否 |
| 47 | {square, nsfw, landscape} | {square, anime} | ✅ square | — | 是 |
| 48 | {square, nsfw, landscape} | {\_\_uncategorized\_\_} | ❌ | ✅ square/nsfw exclusive → 排除 | 否 |
| 49 | {horizontal, vertical} | {horizontal} | ✅ horizontal | — | 是 |
| 50 | {horizontal, vertical} | {vertical} | ✅ vertical | — | 是 |
| 51 | {horizontal, vertical} | {square} | ❌ | — | 否 |
| 52 | {horizontal, vertical} | {nsfw} | ❌ | — | 否 |
| 53 | {horizontal, vertical} | {square, horizontal} | ✅ horizontal | — | 是 |
| 54 | {horizontal, vertical} | {nsfw, vertical} | ✅ vertical | — | 是 |
| 55 | {horizontal, vertical} | {square, nsfw} | ❌ | — | 否 |
| 56 | {horizontal, vertical} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | 是 |
| 57 | {square, horizontal} | {square} | ✅ square | — | 是 |
| 58 | {square, horizontal} | {horizontal} | ✅ horizontal | — | 是 |
| 59 | {square, horizontal} | {square, horizontal} | ✅ both | — | 是 |
| 60 | {square, horizontal} | {vertical} | ❌ | — | 否 |
| 61 | {square, horizontal} | {nsfw} | ❌ | — | 否 |
| 62 | {square, horizontal} | {\_\_uncategorized\_\_} | ❌ | ✅ square.exclusive=true → 排除 | 否 |
| 63 | {nsfw, vertical} | {nsfw} | ✅ nsfw | — | 是 |
| 64 | {nsfw, vertical} | {vertical} | ✅ vertical | — | 是 |
| 65 | {nsfw, vertical} | {square} | ❌ | — | 否 |
| 66 | {nsfw, vertical} | {\_\_uncategorized\_\_} | ❌ | ✅ nsfw.exclusive=true → 排除 | 否 |
| 67 | {square, nsfw, horizontal, vertical} | {square, horizontal} | ✅ square | — | 是 |
| 68 | {square, nsfw, horizontal, vertical} | {nsfw, vertical} | ✅ nsfw/vertical | — | 是 |
| 69 | {square, nsfw, horizontal, vertical} | {anime} | ❌ | — | 否 |
| 70 | {square, nsfw, horizontal, vertical} | {\_\_uncategorized\_\_} | ❌ | ✅ square/nsfw exclusive → 排除 | 否 |
| 71 | {photography} | {photography} | ✅ photography | — | 是 |
| 72 | {photography} | {square} | ❌ | — | 否 |
| 73 | {photography} | {nsfw} | ❌ | — | 否 |
| 74 | {photography} | {\_\_uncategorized\_\_} | ❌ | |Q|=1 严格匹配 | 否 |
| 75 | {square, nsfw, anime, landscape, photography} | {square} | ✅ square | — | 是 |
| 76 | {square, nsfw, anime, landscape, photography} | {nsfw} | ✅ nsfw | — | 是 |
| 77 | {square, nsfw, anime, landscape, photography} | {anime, landscape} | ✅ both | — | 是 |
| 78 | {square, nsfw, anime, landscape, photography} | {square, landscape} | ✅ square | — | 是 |
| 79 | {square, nsfw, anime, landscape, photography} | {horizontal} | ❌ | — | 否 |
| 80 | {square, nsfw, anime, landscape, photography} | {\_\_uncategorized\_\_} | ❌ | ✅ square/nsfw exclusive → 排除 | 否 |
| 81 | {anime, photography} | {anime} | ✅ anime | — | 是 |
| 82 | {anime, photography} | {photography} | ✅ photography | — | 是 |
| 83 | {anime, photography} | {square} | ❌ | — | 否 |
| 84 | {anime, photography} | {nsfw, anime} | ✅ anime | — | 是 |
| 85 | {anime, photography} | {square, nsfw} | ❌ | — | 否 |
| 86 | {anime, photography} | {\_\_uncategorized\_\_} | ❌ | ❌ 无 exclusive tag | 是 |
| 87 | {square, nsfw, horizontal} | {anime, landscape, photography} | ❌ | — | 否 |
| 88 | {}（无选中） | {square} | N/A | — | 是 |
| 89 | {} | {nsfw} | N/A | — | 是 |
| 90 | {} | {anime, landscape} | N/A | — | 是 |
| 91 | {} | {\_\_uncategorized\_\_} | N/A | — | 是 |
