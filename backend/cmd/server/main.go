package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/redis/go-redis/v9"
)

// Redis操作に必要なコンテキスト（お約束の記述です）
var ctx = context.Background()

func main() {
	fmt.Println("バックエンドサーバーを起動中...")

	// 1. Redisクライアントの設定
	rdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379", // ← ここを "localhost:6379" から変更！
		Password: "",
		DB:       0,
	})

	// 2. 疎通確認（PINGを送信）
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		fmt.Println("❌ Redis接続エラー:", err)
	} else {
		fmt.Println("✅ Redis接続成功! 応答:", pong)
	}

	// 3. 簡単なWebサーバー（ブラウザからも確認できるようにします）
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			fmt.Fprintf(w, "Redis接続失敗... %v", err)
		} else {
			fmt.Fprintf(w, "Redis接続成功！ Redisからの返事: %s", pong)
		}
	})

	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
