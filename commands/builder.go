package commands

import (
	"newer_helper/commands/defs"
	"newer_helper/model"

	"github.com/bwmarrin/discordgo"
)

func GenerateCommands(_ *model.ServerConfig) []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		defs.ManageAutoTrigger,
		defs.Punish,
		defs.QuickPunish,
		defs.ResetPunishCooldown,
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
		defs.PunishSearch,
		defs.PunishRevoke,
		defs.PunishDelete,
		defs.PunishPrintEvidence,
		defs.RegisterTopChannel,
		defs.AdsBoardAdmin,
		defs.DailyPunishmentStats,
		defs.SearchPreset,
		defs.GuildsAdmin,
		defs.QuickPreset,
		defs.QuickPresetReplyForAPP,
	}
}
