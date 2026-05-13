# バトルロイヤル麻雀 要件定義書

## 1. プロジェクト概要

### 目的
Go言語の学習・ポートフォリオとして、麻雀をベースにしたバトルロイヤル形式のオンライン対戦ゲームを開発する。就職・転職活動でのアピールを目的とする。

### 特徴
- Go の goroutine / channel / WebSocket を活用したリアルタイム通信
- 麻雀ルールをベースにしたバトルロイヤル独自ルールの追加
- MVP 優先で段階的に開発を進める

---

## 2. 技術スタック

### バックエンド（Go）

| 役割 | ライブラリ |
|---|---|
| HTTP API | `gin-gonic/gin` または `labstack/echo` |
| WebSocket | `gorilla/websocket` |
| 認証 | `golang-jwt/jwt` |
| DB ORM | `uptrace/bun` または `sqlc` |
| Redis クライアント | `redis/go-redis/v9` |
| 設定管理 | `spf13/viper` |
| ホットリロード | `air-verse/air` |

### フロントエンド（Next.js）

| 役割 | ライブラリ |
|---|---|
| フレームワーク | Next.js（App Router / TypeScript / Tailwind） |
| WebSocket 管理 | `reconnecting-websocket` |
| 状態管理 | `zustand` |
| ゲーム描画 | Canvas API または `PixiJS` |
| UI コンポーネント | `shadcn/ui` |

### インフラ（ローカル開発）

| 役割 | 構成 |
|---|---|
| DB | PostgreSQL 16（Docker） |
| キャッシュ / Pub/Sub | Redis 7（Docker） |
| Go サーバー | ホストで直接起動（air でホットリロード） |
| Next.js | ホストで直接起動 |

---

## 3. ディレクトリ構成

```
mahjong-battle/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── game/     # ゲームロジック（牌・アガり判定・役・点数計算）
│   │   ├── room/     # ルーム・goroutine 管理
│   │   ├── ws/       # WebSocket 処理
│   │   └── api/      # HTTP ハンドラ
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── app/
│   ├── package.json
│   └── ...
└── docker-compose.yml
```

> `internal/` 配下に置くことで、パッケージを外部から直接インポートできなくなる（Go の可視性制御）。

---

## 4. ゲーム仕様

### 4-1. 基本ルール
- 麻雀の標準ルール（4人卓）をベースとする
- 4〜8 人でバトルロイヤル形式で対戦

### 4-2. バトルロイヤル独自ルール

#### 必須（MVP に含める）

**点数 0 以下で即脱落 ＋ 持ち点の引き継ぎ**
- 放銃した瞬間に点数が 0 を下回ったプレイヤーは即脱落
- 残った点数はアガったプレイヤーに加算

**全卓リアルタイムランキング表示**
- 自分の卓以外の戦況も右サイドバーに表示
- 全員の点数を Redis Pub/Sub でブロードキャスト

#### Phase 2 以降

**持ち時間制（タイムプレッシャー）**
- 各プレイヤーに持ち時間（例：5 分）を設定
- 時間切れで自動ツモ切り、さらに時間切れで強制脱落
- `time.Timer` と goroutine の組み合わせで実装

#### Phase 3 以降

**スキルカード（アビリティシステム）**
- アガるたびにランダムなスキルを 1 枚獲得
- 例：「他プレイヤーの手牌を 1 枚公開」「次局の親を奪う」など
- Strategy パターンで拡張しやすく設計

### 4-3. ゲームの流れ（MVP 版）

1. ロビーに 4〜8 人が集合
2. 自動で 4 人卓に分割
3. 通常の麻雀ルールで局を進行
4. 点数 0 以下のプレイヤーを脱落と判定
5. 生存者を再集結させて卓を再編成
6. 最終的に残った 4 人で決戦

---

## 5. システムアーキテクチャ

### 5-1. 全体構成

```
[Next.js] ──HTTP──▶ [Go HTTP API サーバー]
                         認証 / マッチング / ルーム管理
[Next.js] ──WS──▶  [Go WebSocket サーバー]
                         ゲームルーム / goroutine / channel
                              │
                    ┌─────────┴─────────┐
                 [Redis]          [PostgreSQL]
          ゲーム状態 / Pub/Sub    ユーザー / 履歴 / ランキング
```

### 5-2. ゲームループ設計（1 ルーム = 1 goroutine）

```go
type Room struct {
    players     map[string]*Player
    inputCh     chan InputMsg    // プレイヤー入力を受け取る
    broadcastCh chan GameState   // 全員に送る
    state       GameState
}

func (r *Room) Run() {
    ticker := time.NewTicker(time.Second / 60) // 60fps
    for {
        select {
        case input := <-r.inputCh:
            r.applyInput(input)
        case <-ticker.C:
            r.updateState()
            r.broadcastCh <- r.state
        }
    }
}
```

> mutex なしでゲーム状態を安全に管理できる。channel が排他制御を自然に解決する。

### 5-3. 主要データ構造

```go
type BattleRoom struct {
    ID      string
    Tables  []*Table           // 複数の卓
    Players map[string]*Player
    Alive   []string           // 生存プレイヤー ID
    EventCh chan GameEvent      // 脱落・アガりイベント
}

type Player struct {
    ID      string
    Score   int                // 0 以下で脱落
    Hand    []Tile
    Conn    *websocket.Conn
    IsAlive bool
}
```

---

## 6. 開発フェーズ（ロードマップ）

### Phase 1：ゲームエンジン（2〜3 週間）

WebSocket やフロントは触らず、純粋なゲームロジックのみを `internal/game/` に実装する。

**実装順序**

1. **牌のデータ構造**（`tile.go`）
   - `Suit`（万子 / 筒子 / 索子 / 字牌）と `Tile` 型の定義
   - 136 枚の牌山生成・シャッフル
   - `IsRed bool` フィールドを最初から用意（赤ドラ対応）

2. **手牌管理**（`hand.go`）
   - `Hand` / `Meld` 型の定義
   - ツモ・打牌・ソートの実装

3. **アガり判定**（`win.go`）
   - 七対子・国士無双・通常形（再帰探索）
   - 通常形は「雀頭を仮定 → メンツを再帰的に抜き出す」アルゴリズム

4. **役判定・点数計算**（`yaku.go`）
   - `Yaku` インターフェースで拡張しやすく設計
   - MVP で実装する役：タンヤオ / リーチ / ピンフ / ツモ / イーペーコー

**Phase 1 完了基準**

- 136 枚の牌山が正しく生成できる
- アガり判定が主要パターンで正しく動く（`go test` 通過）
- MVP 役 5 種の判定が正しく動く
- 点数計算が基本パターンで正しく動く

### Phase 2：WebSocket ＋ バトルロイヤル機能（1〜2 週間）

- `1 卓 = 1 goroutine` でゲームループを実装
- 脱落イベントを `BattleRoom.EventCh` に流して卓再編成を処理
- Redis Pub/Sub で全卓のスコアをリアルタイムブロードキャスト
- 切断復帰対応（ゲーム状態を Redis に保持）

### Phase 3：フロントエンド（1 週間）

- Next.js でゲーム盤面とリアルタイムランキングを実装
- ゲームロジックはすべて Go 側に持たせ、フロントは UI 表示に集中

---

## 7. 採用担当へのアピールポイント

| ポイント | 内容 |
|---|---|
| アガり判定ロジック | 再帰的な探索アルゴリズムの実装経験として面接で話せる |
| 3 層設計 | `BattleRoom → Table → Player` の設計は実務のサービス設計に直結 |
| 並行処理 | 複数卓をまたいだリアルタイムランキングで Redis Pub/Sub ＋ goroutine を組み合わせた本格的な実装 |
| テスト | 各 Step ごとにユニットテストを記述、カバレッジ 80% 以上を目指す |
| CI/CD | GitHub Actions でテスト自動化・本番デプロイ（Railway / Fly.io） |

---

## 8. 環境構築手順（完了済み）

1. Docker Desktop インストール → `docker-compose up` で Redis・PostgreSQL 起動確認
2. Go インストール（1.22 以上）→ `go mod init` でモジュール作成
3. `air` インストール → ホットリロード確認
4. Next.js セットアップ → `npm run dev` で起動確認
5. Go サーバーから Redis への疎通確認（Ping）
