package handler

import (
	"encoding/json"
	"slices"
	"strconv"
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
		if cmd.Mode == Async && cmd.Work == nil {
			t.Errorf("%s: Async command has no Work", name)
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

	got := make([]string, 0, len(defs))
	for _, d := range defs {
		got = append(got, d.CommandName())
	}

	// mapの反復順に引きずられると、登録内容が実行ごとに変わってしまう。
	want := []string{"help", "ping", "setup"}
	if !slices.Equal(got, want) {
		t.Errorf("definitions = %v, want %v (sorted by name)", got, want)
	}
}

// commandDefinition は登録時に送られる定義のうち、検証したい部分。
type commandDefinition struct {
	Contexts                 []int  `json:"contexts"`
	IntegrationTypes         []int  `json:"integration_types"`
	DefaultMemberPermissions string `json:"default_member_permissions"`
}

// definitionOf は名前でコマンド定義の中身を引く。
func definitionOf(t *testing.T, name string) commandDefinition {
	t.Helper()

	def := Registry[name].Definition
	if def == nil {
		t.Fatalf("%s has no definition", name)
	}
	raw, err := json.Marshal(*def)
	if err != nil {
		t.Fatal(err)
	}

	var got commandDefinition
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

// setupはWebhookを作るのでサーバー限定にする。DMでは作れず機能しない。
func TestSetupIsGuildOnly(t *testing.T) {
	got := definitionOf(t, "setup")

	if len(got.Contexts) != 1 || got.Contexts[0] != int(dgo.InteractionContextTypeGuild) {
		t.Errorf("contexts = %v, want [%d]", got.Contexts, dgo.InteractionContextTypeGuild)
	}
	if len(got.IntegrationTypes) != 1 ||
		got.IntegrationTypes[0] != int(dgo.ApplicationIntegrationTypeGuildInstall) {
		t.Errorf("integration_types = %v, want [%d]",
			got.IntegrationTypes, dgo.ApplicationIntegrationTypeGuildInstall)
	}
}

// 一般ユーザーが実行できないよう、サーバー管理権限を要求する。
func TestSetupRequiresManageGuild(t *testing.T) {
	got := definitionOf(t, "setup")

	want := strconv.Itoa(int(dgo.PermissionManageGuild))
	if got.DefaultMemberPermissions != want {
		t.Errorf("default_member_permissions = %q, want %q (Manage Guild)",
			got.DefaultMemberPermissions, want)
	}
}

// 非推奨のdm_permissionを送らない。ContextsとIntegrationTypesで指定する。
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
