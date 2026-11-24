package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_UpdateChannels(t *testing.T) {
	svc, _ := setupService(t)
	b, err := svc.Create("渠道机器人", "", "", "", "")
	require.NoError(t, err)

	// 空绑定
	channels, err := svc.GetChannels(b.ID)
	require.NoError(t, err)
	assert.Empty(t, channels)

	// 正常设置
	_, err = svc.UpdateChannels(b.ID, []BotChannel{
		{Platform: " Telegram ", ChatID: " -100123 "},
		{Platform: "wecom", UserID: "u-1"},
	})
	require.NoError(t, err)
	channels, err = svc.GetChannels(b.ID)
	require.NoError(t, err)
	require.Len(t, channels, 2)
	assert.Equal(t, "telegram", channels[0].Platform)
	assert.Equal(t, "-100123", channels[0].ChatID)

	// 校验：非法平台
	_, err = svc.UpdateChannels(b.ID, []BotChannel{{Platform: "feishu", ChatID: "1"}})
	assert.ErrorContains(t, err, "无效平台")

	// 校验：chat_id 与 user_id 同时为空
	_, err = svc.UpdateChannels(b.ID, []BotChannel{{Platform: "telegram"}})
	assert.ErrorContains(t, err, "不能同时为空")
}

func TestService_ResolveBotByChannel(t *testing.T) {
	svc, _ := setupService(t)

	b1, err := svc.Create("群机器人", "客服群", "", "", "")
	require.NoError(t, err)
	_, err = svc.UpdateChannels(b1.ID, []BotChannel{
		{Platform: "telegram", ChatID: "-100123"},
	})
	require.NoError(t, err)

	b2, err := svc.Create("私聊机器人", "1v1 助手", "", "", "")
	require.NoError(t, err)
	_, err = svc.UpdateChannels(b2.ID, []BotChannel{
		{Platform: "telegram", UserID: "user-42"},
	})
	require.NoError(t, err)

	// 命中 chat_id
	hit := svc.ResolveBotByChannel("telegram", "-100123", "someone-else")
	require.NotNil(t, hit)
	assert.Equal(t, b1.ID, hit.ID)

	// 命中 user_id
	hit = svc.ResolveBotByChannel("telegram", "other-chat", "user-42")
	require.NotNil(t, hit)
	assert.Equal(t, b2.ID, hit.ID)

	// 平台不匹配
	assert.Nil(t, svc.ResolveBotByChannel("wecom", "-100123", ""))

	// 未命中
	assert.Nil(t, svc.ResolveBotByChannel("telegram", "no-chat", "no-user"))

	// 禁用后不再参与路由
	disabled := false
	_, err = svc.Update(b1.ID, nil, nil, nil, nil, &disabled)
	require.NoError(t, err)
	assert.Nil(t, svc.ResolveBotByChannel("telegram", "-100123", ""))
}

func TestBotChannelResolver_ToBotInfo(t *testing.T) {
	svc, _ := setupService(t)
	b, err := svc.Create("信息机器人", "职责描述", "qwen3", "prov-1", "")
	require.NoError(t, err)
	_, err = svc.UpdateChannels(b.ID, []BotChannel{{Platform: "webhook", ChatID: "c1"}})
	require.NoError(t, err)

	resolver := &botChannelResolver{service: svc}
	info := resolver.ResolveBotByChannel("webhook", "c1", "")
	require.NotNil(t, info)
	assert.Equal(t, b.ID, info.ID)
	assert.Equal(t, "信息机器人", info.Name)
	assert.Equal(t, "职责描述", info.Description)
	assert.Equal(t, "qwen3", info.Model)
	assert.Equal(t, "prov-1", info.ProviderID)

	assert.Nil(t, resolver.ResolveBotByChannel("webhook", "c2", ""))
}
