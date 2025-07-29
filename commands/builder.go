package commands

import (
	"discord-bot/commands/defs"
	"discord-bot/model"

	"github.com/bwmarrin/discordgo"
)

func GenerateCommands(_ *model.ServerConfig) []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		defs.ManageAutoTrigger,
		defs.Punish,
		defs.QuickPunish,
		defs.PresetMessage,
		defs.PresetMessageUpd,
		defs.PresetMessageAdmin,
		defs.Rollcard,
		defs.StartScan,
		defs.NewCards,
		defs.SetupRollPanel,
		defs.SystemInfo,
		defs.ReloadConfig,
		defs.NewPostPushAdmin,
		defs.PunishAdmin,
		defs.RegisterTopChannel,
		defs.AdsBoardAdmin,
		defs.DailyPunishmentStats,
		defs.SearchPreset,
		defs.GuildsAdmin,
	}
}
