package rollcard

import (
	"discord-bot/bot"
	"discord-bot/model"
	"discord-bot/utils"
	"discord-bot/utils/database"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func HandleGlobalRoll(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, customID string) {
	parts := strings.SplitN(customID, ":", 2)
	if len(parts) != 2 {
		log.Printf("Invalid customID format for global roll: %s", customID)
		return
	}

	count, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Printf("Invalid count for global roll: %s", parts[1])
		return
	}

	globalRollCard(s, i, b, count, "", nil)
}

func globalRollCard(s *discordgo.Session, i *discordgo.InteractionCreate, b *bot.Bot, count int, tagID string, excludeTags []string) {
	if tagID != "" || len(excludeTags) > 0 {
		utils.SendEphemeralResponse(s, i, "全局抽卡模式不支持使用标签。")
		return
	}

	userID := i.Member.User.ID
	log.Printf("Executing globalRollCard for user %s. Count: %d", userID, count)

	posts, err := getGlobalPosts(b.GetConfig(), count)
	if err != nil {
		log.Printf("Error getting global posts: %v", err)
		utils.SendEphemeralResponse(s, i, err.Error())
		return
	}

	if len(posts) == 0 {
		utils.SendEphemeralResponse(s, i, "卡池为空，或未找到符合条件的卡片 ")
		return
	}

	embeds := buildEmbeds(posts, nil, i.Member.User.Username)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: embeds,
			Flags:  discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "再来一抽 (全局)",
							Style:    discordgo.PrimaryButton,
							CustomID: "global_roll:1",
						},
					},
				},
			},
		},
	})
}

func getGlobalPosts(config *model.Config, count int) ([]model.Post, error) {
	var allPosts []model.Post
	for _, guildConfig := range config.RollCardConfigs {
		db, err := database.InitDB(guildConfig.Database)
		if err != nil {
			log.Printf("Could not initialize database for guild %s: %v", guildConfig.GuildID, err)
			continue
		}
		defer db.Close()

		posts, err := database.GetRandomPostsFromAllTables(db, count*2) // Fetch more to increase variety
		if err != nil {
			log.Printf("Could not get posts from guild %s: %v", guildConfig.GuildID, err)
			continue
		}
		allPosts = append(allPosts, posts...)
	}

	if len(allPosts) == 0 {
		return nil, fmt.Errorf("no posts found in any configured guild")
	}

	// Shuffle all collected posts
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(allPosts), func(i, j int) { allPosts[i], allPosts[j] = allPosts[j], allPosts[i] })

	if len(allPosts) > count {
		return allPosts[:count], nil
	}

	return allPosts, nil
}
