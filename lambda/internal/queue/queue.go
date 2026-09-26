// Package queue はinteraction LambdaからworkerLambdaへInteractionを渡すSQSのやり取りをまとめる。
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// sendTimeout はSQSへの送信に許す時間。
//
// Discordの3秒制限に間に合わなくなると、キューには残っているのに実行者には失敗が見える状態になる。
// 粘らずに諦めてエラーを返す。
const sendTimeout = time.Second

// Message はSQSに積む内容。
//
// Interactionは解釈せず生のまま運ぶ。
// interaction Lambdaに業務ロジックを持ち込まないため。
type Message struct {
	// ReceivedAt はinteraction Lambdaが受け取った時刻。
	// worker側でtokenの有効期限 (15分) を判定するのに使う。
	ReceivedAt time.Time `json:"received_at"`
	// Interaction は受信したInteractionのJSON。
	Interaction json.RawMessage `json:"interaction"`
}

// Sender はSQSへの送信を担う。
type Sender struct {
	client   *sqs.Client
	queueURL string
}

// NewSender は送信側を作る。
func NewSender(ctx context.Context, queueURL string) (*Sender, error) {
	// 既定のリトライに任せると3秒を超えうるので、1回だけ試す設定にする。
	// 待ち時間の制御はsendTimeout側で行う。
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRetryMaxAttempts(1))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Sender{client: sqs.NewFromConfig(cfg), queueURL: queueURL}, nil
}

// Send はInteractionをキューに積む。
//
// rawには受信したままのJSONを渡す。
// groupIDには同じ順序で処理したい単位 (チャンネルID) を、dedupeIDにはInteractionのIDを渡す。
func (s *Sender) Send(ctx context.Context, groupID, dedupeID string, raw []byte) error {
	body, err := json.Marshal(Message{
		ReceivedAt:  time.Now().UTC(),
		Interaction: raw,
	})
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	_, err = s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:               aws.String(s.queueURL),
		MessageBody:            aws.String(string(body)),
		MessageGroupId:         aws.String(groupID),
		MessageDeduplicationId: aws.String(dedupeID),
	})
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}
