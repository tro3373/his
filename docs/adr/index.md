# ADR

## Accepted

### ADR-0001 20261107-133000
- gen 区画は開始と終了の HTML コメント行で囲み、開始行に中身の hash を書く
  - 形: `<!-- his gen <hash> -->` ... `<!-- his gen end -->`
  - 中身が hash と一致する区画だけを、再実行で置き換える
- 却下: hash の状態をログの外のファイルに持つ
  - ログを手で移す・消すと、外の状態と中身がずれる
- 却下: 開始行だけに hash と行数を書く
  - 区画の終わりが目で見て分からない

## Under Consideration


## Deprecated
