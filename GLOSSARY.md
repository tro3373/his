# his

Markdown の時間ログを読み書きし、tag と題名ごとに稼働を集計する。

## Language

**日の節**:
`## YYYY-MM-DD (曜)` から次の日の見出しまでの、1 日分のログ。

**時間見出し**:
`### YYYYMMDD_HHMMSS` で始まり、その時刻から次の時間見出しまでの稼働を表す行。

**gen 区画**:
`his gen` が日の節に書いた、時間見出しと task link のまとまり。書いたときの hash を伴う。
_Avoid_: block, gen 出力

**手修正**:
gen 区画の中身が、書いたときの hash と合わなくなった状態。hash の無い区画も含む。
