package bond

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	TG_UPDATE_ID_KEY = "TELEGRAM_UPDATE_ID"
	TG_CHAT_ID_KEY   = "TELEGRAM_CHAT_ID"
	TG_PARSE_MODE    = "Markdown"
	JISILU_KEY       = "JISILU"
	JISILU_URL       = "https://www.jisilu.cn/data/calendar/get_calendar_data/?qtype=CNV"
	DURATION         = 1209600 // 14 * 24 * 60 * 60
)

type bond struct {
	ID    string `json:"id"`
	Code  string `json:"code"`
	Title string `json:"title"`
	Start string `json:"start"`
}

// Process is the main func to interacte with telegram bot
func Process() {

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TGBOT_TOKEN"))
	if err != nil {
		log.Printf("E! NewBotAPI failed: %v", err)
		os.Exit(1)
	}
	bot.Debug = true

	// get latest update id
	updateID, err := redisClient.Get(TG_UPDATE_ID_KEY).Result()
	if err != nil {
		log.Printf("W! fail to get update_id")
	}
	offset, err := strconv.Atoi(updateID)
	if err != nil {
		log.Printf("W! fail to convert update_id to int")
	}
	u := tgbotapi.NewUpdate(offset + 1)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Printf("E! GetUpdatesChan: %s", err.Error())
	}
	for update := range updates {
		if update.Message == nil {
			continue
		}
		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		chatIDInString := strconv.FormatInt(update.Message.Chat.ID, 10)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		msg.ParseMode = TG_PARSE_MODE
		msg.Text = ""
		addChatID(chatIDInString)

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				msg.Text = "- 添加可转债: `/add 转债名`\n- 删除可转债: `/rm 转债名`\n- 显示已添加可转债: `/list`\n- 显示近期可转债: `/coming`\n\n[了解更多](https://github.com/xuqingfeng/BondReminderBot)"
			case "add":
				addCustomConvertibleBond(chatIDInString, update.Message.CommandArguments())
				msg.Text = "✔"
			case "rm":
				removeCustomConvertibleBond(chatIDInString, update.Message.CommandArguments())
				msg.Text = "✔"
			case "list":
				result, err := listCustomConvertibleBond(chatIDInString)
				if err != nil {
					log.Printf("E! redis smembers failed: %v", err)
				} else {
					msg.Text = formatCustomConvertibleBond(result)
				}
			case "coming":
				// get future bonds
				result, err := getFutureBonds(false)
				if err != nil {
					log.Printf("E! getFutureBonds failed: %v", err)
					msg.Text = "⚠️ 出错信息: " + err.Error()
				} else {
					msg.Text = formatFutureBonds(result)
				}
			default:
				msg.Text = "⚠️ 未能识别指令"
			}
		} else {
			msg.Text = "⚠️ 请输入指令"
		}
		if msg.Text != "" {
			bot.Send(msg)
		}
		redisClient.Set(TG_UPDATE_ID_KEY, update.UpdateID, 0)
	}
}

func listCustomConvertibleBond(chatID string) ([]string, error) {

	result, err := redisClient.SMembers(chatID).Result()
	return result, err
}

func formatCustomConvertibleBond(bonds []string) string {

	var isEmpty = true
	var result = "*已保存可转债:*\n```\n"
	for _, bond := range bonds {
		result = result + bond + "\n"
		isEmpty = false
	}
	if isEmpty {
		return "暂无相关信息"
	}
	return result + "```"
}

func addCustomConvertibleBond(chatID string, bond string) {

	redisClient.SAdd(chatID, bond)
}

func removeCustomConvertibleBond(chatID string, bond string) {
	redisClient.SRem(chatID, bond)
}

func addChatID(chatID string) {
	redisClient.SAdd(TG_CHAT_ID_KEY, chatID)
}

func listChatID() ([]string, error) {
	result, err := redisClient.SMembers(TG_CHAT_ID_KEY).Result()
	return result, err
}

// FetchFutureBonds will call jisilu.cn to get future bonds info
func FetchFutureBonds() error {

	start := time.Now().Unix()
	end := start + DURATION
	apiURL := fmt.Sprintf(JISILU_URL+"&start=%s&end=%s", strconv.FormatInt(start, 10), strconv.FormatInt(end, 10))
	resp, err := http.Get(apiURL)
	if err != nil {
		log.Printf("E! call jisilu.cn failed: %v", err)
		return err
	}
	if resp.StatusCode == http.StatusOK {
		bonds := new([]bond)
		err = json.NewDecoder(resp.Body).Decode(bonds)
		log.Printf("bonds: %v", bonds)
		if err != nil {
			log.Printf("E! decode json response failed: %v", err)
			return err
		} else {
			bondsInStr, err := json.Marshal(bonds)
			if err != nil {
				log.Printf("E! encode json failed: %v", err)
				return err
			} else {
				// save json as string in redis
				redisClient.Set(JISILU_KEY, bondsInStr, 0)
				return nil
			}
		}
	}
	return errors.New(resp.Status)
}

func getFutureBonds(fullList bool) ([]bond, error) {

	bonds, err := redisClient.Get(JISILU_KEY).Result()
	if err != nil {
		return nil, err
	}

	bondsInJSON := new([]bond)
	err = json.Unmarshal([]byte(bonds), bondsInJSON)
	if err != nil {
		return nil, err
	}

	if !fullList {
		var filteredBonds []bond
		for _, b := range *bondsInJSON {
			if strings.Contains(b.Title, "上市日") || strings.Contains(b.Title, "申购日") {
				filteredBonds = append(filteredBonds, b)
			}
		}
		return filteredBonds, nil
	}

	return *bondsInJSON, nil
}
func formatFutureBonds(bonds []bond) string {

	var isEmpty = true
	var result = "*近期可转债信息:*\n```\n"
	for _, bond := range bonds {
		result = result + bond.Title + " " + bond.Start + "\n"
		isEmpty = false
	}
	if isEmpty {
		return "暂无相关信息"
	}
	return result + "```"
}

// 打新提醒
func formatTodayBonds(bonds []bond) string {

	var isEmpty = true
	var today = time.Now().Format("2006-01-02")
	var result = "*打新提醒:*\n```\n"
	for _, bond := range bonds {
		if strings.Contains(bond.Title, "申购日") && bond.Start == today {
			result = result + bond.Title + " " + bond.Start + "\n"
			isEmpty = false
		}
	}
	if isEmpty {
		return ""
	}
	return result + "```"
}

func getBondsInWatchlist(customBonds []string, bonds []bond) []bond {

	var result []bond
	for _, b := range bonds {
		for _, c := range customBonds {
			s := []rune(c)
			// match first 2 characters(unicode) - https://stackoverflow.com/a/26166654/4036946
			if strings.Contains(b.Title, string(s[0:2])) { // e.g. 东财A, 东财B
				// replace bond title with saved name
				b.Title = c
				result = append(result, b)
			}
		}
	}
	return result
}

// 上市提醒
func formatWantedBonds(bonds []bond) string {

	var isEmpty = true
	var result = "*上市提醒:*\n```\n"
	for _, bond := range bonds {
		result = result + bond.Title + " " + bond.Start + "\n"
		isEmpty = false
	}
	if isEmpty {
		return ""
	}
	return result + "```"
}

// Notify is used to send notification msg per chat id
func Notify() error {

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TGBOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	// get future bonds
	futureBonds, err := getFutureBonds(true)
	if err != nil {
		return err
	}

	// loop chat id
	chatIDs, err := listChatID()
	log.Printf("I! chatIDs: %v", chatIDs)
	if err != nil {
		return err
	}
	// TODO: enable rate limit
	for _, chatID := range chatIDs {
		customBonds, err := listCustomConvertibleBond(chatID)
		if err != nil {
			return err
		}
		wantedBonds := getBondsInWatchlist(customBonds, futureBonds)
		todayBonds := formatTodayBonds(futureBonds)
		chatIDInInt64, _ := strconv.ParseInt(chatID, 10, 64)
		msg := tgbotapi.NewMessage(chatIDInInt64, "")
		msg.ParseMode = TG_PARSE_MODE
		if todayBonds != "" || len(wantedBonds) != 0 {
			msg.Text = todayBonds + "\n" + formatWantedBonds(wantedBonds)
			bot.Send(msg)
		}
	}
	return nil
}
