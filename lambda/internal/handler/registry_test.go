package handler

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	dgo "github.com/disgoorg/disgo/discord"
)

// slashCommand はスラッシュコマンドのInteractionを組み立てる。
func slashCommand(t *testing.T, name string) dgo.Interaction {
	t.Helper()

	raw := `{"id":"1","application_id":"2","type":2,"token":"t","version":1,` +
		`"data":{"id":"3","name":"` + name + `","type":1},` +
		`"channel":{"id":"4","type":0},"user":{"id":"5","username":"u","discriminator":"0"}}`

	i, err := dgo.UnmarshalInteraction([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return i
}

// SyncとAsyncで埋めるべきフィールドが入れ替わるため、取り違えを検出する。
func TestRegistryEntriesAreConsistent(t *testing.T) {
	for name, cmd := range Registry {
		if cmd.Mode == Sync && cmd.Handle == nil {
			t.Errorf("%s: Sync command has no Handle", name)
		}
		if cmd.Mode == Async && cmd.Handle != nil {
			t.Errorf("%s: Async command must not have Handle", name)
		}
	}
}

func TestLookup(t *testing.T) {
	for _, name := range []string{"ping", "help"} {
		t.Run(name, func(t *testing.T) {
			if _, ok := Lookup(slashCommand(t, name)); !ok {
				t.Errorf("Lookup(%q) found nothing", name)
			}
		})
	}

	if _, ok := Lookup(slashCommand(t, "nosuchcommand")); ok {
		t.Error("Lookup found a handler for an unknown command")
	}
}

func TestDefinitions(t *testing.T) {
	defs := Definitions()
	if len(defs) != 2 {
		t.Fatalf("got %d definitions, want 2", len(defs))
	}

	// mapの反復順に引きずられると、登録内容が実行ごとに変わってしまう。
	if defs[0].CommandName() != "help" || defs[1].CommandName() != "ping" {
		t.Errorf("definitions are not sorted by name: %s, %s",
			defs[0].CommandName(), defs[1].CommandName())
	}
}

// definitionOf は名前でコマンド定義の中身を引く。
func definitionOf(t *testing.T, name string) struct {
	Contexts                 []int  `json:"contexts"`
	IntegrationTypes         []int  `json:"integration_types"`
	DefaultMemberPermissions string `json:"default_member_permissions"`
} {
	t.Helper()

	var got struct {
		Contexts                 []int  `json:"contexts"`
		IntegrationTypes         []int  `json:"integration_types"`
		DefaultMemberPermissions string `json:"default_member_permissions"`
	}

	def := Registry[name].Definition
	if def == nil {
		t.Fatalf("%s has no definition", name)
	}
	raw, err := json.Marshal(*def)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// ContextsとIntegrationTypesを必ず明示すること。
//
// 省略するとDiscordの既定に従い、意図しない場所で使えてしまう。
// 新しいコマンドを足すとき、どこで使えるかを考え忘れたことを検知する。
func TestDefinitionsDeclareWhereTheyCanBeUsed(t *testing.T) {
	for _, def := range Definitions() {
		name := def.CommandName()
		got := definitionOf(t, name)

		if len(got.Contexts) == 0 {
			t.Errorf("%s: contexts is empty", name)
		}
		if len(got.IntegrationTypes) == 0 {
			t.Errorf("%s: integration_types is empty", name)
		}
		// このBotの主な用途はサーバー内。Guildが外れていたら設定ミス。
		if !slices.Contains(got.Contexts, int(dgo.InteractionContextTypeGuild)) {
			t.Errorf("%s: contexts = %v, want it to include the guild context", name, got.Contexts)
		}
	}
}

// 非推奨のdm_permissionを送らない。Contextsで指定する。
func TestDefinitionsDoNotUseDMPermission(t *testing.T) {
	for _, def := range Definitions() {
		raw, err := json.Marshal(def)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "dm_permission") {
			t.Errorf("%s: definition contains dm_permission: %s", def.CommandName(), raw)
		}
	}
}

func TestPingReturnsPong(t *testing.T) {
	resp, err := ping.Handle(slashCommand(t, "ping"))
	if err != nil {
		t.Fatal(err)
	}

	if resp.Type != dgo.InteractionResponseTypeCreateMessage {
		t.Errorf("type = %d, want %d", resp.Type, dgo.InteractionResponseTypeCreateMessage)
	}

	data, ok := resp.Data.(dgo.MessageCreate)
	if !ok {
		t.Fatalf("data = %T, want MessageCreate", resp.Data)
	}
	if data.Content != "pong" {
		t.Errorf("content = %q, want pong", data.Content)
	}
}

// helpMessage はhelpの応答からMessageCreateを取り出す。
func helpMessage(t *testing.T) dgo.MessageCreate {
	t.Helper()

	resp, err := help.Handle(slashCommand(t, "help"))
	if err != nil {
		t.Fatal(err)
	}

	data, ok := resp.Data.(dgo.MessageCreate)
	if !ok {
		t.Fatalf("data = %T, want MessageCreate", resp.Data)
	}
	return data
}

// helpは実行者にだけ見せる。チャンネルに使い方が流れないようにする。
func TestHelpIsEphemeral(t *testing.T) {
	if flags := helpMessage(t).Flags; flags&dgo.MessageFlagEphemeral == 0 {
		t.Errorf("flags = %d, want the ephemeral flag to be set", flags)
	}
}
