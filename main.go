package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	appId     = os.Getenv("APP_ID")
	appSecret = os.Getenv("APP_SECRET")
	chatId    = os.Getenv("CHAT_ID")
	userEmail = os.Getenv("USER_EMAIL")
	// userId    = os.Getenv("USER_ID")
	interval = os.Getenv("INTERVAL") //minutes
)

func main() {
	// Get Token
	token, err := genTenantAccessToken(appId, appSecret)
	if err != nil {
		fmt.Println(err)
		return
	}

	//通过chatID获取历史消息 message_id
	historyMessagesID, err := getHistoryMessage(chatId, token)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("The latest messageID: ", historyMessagesID)

	//查询最新消息已读信息
	ifRead, err := checkMessageStatus(historyMessagesID, token)
	if err != nil {
		fmt.Println(err)
		return
	}
	if ifRead {
		fmt.Println("No unread message, skip")
		return
	}

	//根据邮箱获取 user_id
	userId, err := getUserIdByEmail(token)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 发送加急消息
	err = callPhone(historyMessagesID, userId, token)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Alredy call User %s for message %s \n", userId, historyMessagesID)
}

func genTenantAccessToken(appId, appSecret string) (string, error) {
	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	method := "POST"
	payload := strings.NewReader("{\"app_id\": \"" + appId + "\", \"app_secret\": \"" + appSecret + "\"}")
	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		// fmt.Println(err)
		return "", err
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	jsonData := make(map[string]interface{})
	json.NewDecoder(res.Body).Decode(&jsonData)
	if jsonData["code"].(float64) != 0 {
		err, _ := jsonData["msg"].(string)
		return "", fmt.Errorf(err)
	}
	// fmt.Println("body: ", jsonData)

	token, ok := jsonData["tenant_access_token"]
	if !ok {
		return "", fmt.Errorf("Get tenant_access_token faild")
	}
	return token.(string), nil
}

func getHistoryMessage(chatId, authToken string, pageToken ...string) (string, error) {
	cstSh, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return "", err
	}
	now := time.Now().In(cstSh)
	intervalTime, _ := time.ParseDuration("-" + interval)
	startTime := fmt.Sprintf("%v", now.Add(intervalTime).Unix())
	endTime := fmt.Sprintf("%v", now.Unix())
	// fmt.Println(startTime, endTime)
	var url string
	if len(pageToken) == 0 {
		url = "https://open.feishu.cn/open-apis/im/v1/messages?container_id=" + chatId + "&container_id_type=chat&end_time=" + endTime + "&page_size=50&start_time=" + startTime
	} else {
		url = "https://open.feishu.cn/open-apis/im/v1/messages?container_id=" + chatId + "&container_id_type=chat&page_size=50&page_token=" + pageToken[0] + "&end_time=" + endTime + "&start_time=" + startTime
	}

	method := "GET"
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+authToken)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	jsonData := make(map[string]interface{})
	json.NewDecoder(res.Body).Decode(&jsonData)
	if jsonData["code"].(float64) != 0 {
		err, _ := jsonData["msg"].(string)
		return "", fmt.Errorf(err)
	}
	// fmt.Println("body: ", jsonData)

	messData, ok := jsonData["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Read history message data faild")
	}

	hasMore, _ := messData["has_more"].(bool)
	if hasMore {
		fmt.Println("Read the next page")
		pageToken, _ := messData["page_token"].(string)
		messageId, err := getHistoryMessage(chatId, authToken, pageToken)
		return messageId, err
	}

	messageItems, ok := messData["items"].([]interface{})
	if !ok {
		return "", fmt.Errorf("Read history message items faild")
	}

	messageLength := len(messageItems)
	if messageLength == 0 {
		return "", fmt.Errorf("There is no new messages, skip")
	}

	fmt.Printf("Get %v messages in the past %v \n", messageLength, interval)
	latestItem, ok := messageItems[messageLength-1].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Read the latest history message data faild")
	}

	// 判断最新消息的发送者是否是机器人
	sender, ok := latestItem["sender"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Read the latest history message sender faild")
	}
	senderId, ok := sender["id"].(string)
	if !ok {
		return "", fmt.Errorf("Read the latest history message sender ID faild")
	}
	if senderId != appId {
		return "", fmt.Errorf("The latest messages are not sent by bots, skip")
	}

	messageId, ok := latestItem["message_id"].(string)
	if !ok {
		return "", fmt.Errorf("Read the latest history message ID faild")
	}
	return messageId, nil
}

func checkMessageStatus(messageId, authToken string) (bool, error) {
	url := "https://open.feishu.cn/open-apis/im/v1/messages/" + messageId + "/read_users?user_id_type=user_id"
	method := "GET"
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Add("Authorization", "Bearer "+authToken)
	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	jsonData := make(map[string]interface{})
	json.NewDecoder(res.Body).Decode(&jsonData)
	if jsonData["code"].(float64) != 0 {
		err, _ := jsonData["msg"].(string)
		return false, fmt.Errorf(err)
	}
	// fmt.Println("body: ", jsonData)

	messData, ok := jsonData["data"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("Read the latest history message status faild")
	}

	items, ok := messData["items"].([]interface{})
	if !ok {
		return false, fmt.Errorf("Read the latest history message items status faild")
	}

	if len(items) == 0 {
		// 消息未读
		return false, nil
	}

	return true, nil
}

func getUserIdByEmail(authToken string) (string, error) {
	url := "https://open.feishu.cn/open-apis/contact/v3/users/batch_get_id?user_id_type=user_id"
	method := "POST"
	payload := strings.NewReader("{\"emails\": [\"" + userEmail + "\"]}")
	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+authToken)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	jsonData := make(map[string]interface{})
	json.NewDecoder(res.Body).Decode(&jsonData)

	if jsonData["code"].(float64) != 0 {
		err, _ := jsonData["msg"].(string)
		return "", fmt.Errorf(err)
	}

	userData, ok := jsonData["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Read the user data faild")
	}

	userList, ok := userData["user_list"].([]interface{})
	if !ok {
		return "", fmt.Errorf("Read the user list faild")
	}

	if len(userList) == 0 {
		return "", fmt.Errorf("found no users for this email")
	}

	user, _ := userList[0].(map[string]interface{})
	return user["user_id"].(string), nil
}

func callPhone(messageId string, userId string, authToken string) error {
	url := "https://open.feishu.cn/open-apis/im/v1/messages/" + messageId + "/urgent_phone?user_id_type=user_id"
	method := "PATCH"
	payload := strings.NewReader("{\"user_id_list\": [\"" + userId + "\"]}")

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+authToken)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	jsonData := make(map[string]interface{})
	json.NewDecoder(res.Body).Decode(&jsonData)
	// fmt.Println("body: ", jsonData)url := "https://open.feishu.cn/open-apis/contact/v3/users/batch_get_id?user_id_type=user_id"
	if jsonData["code"].(float64) != 0 {
		err, _ := jsonData["msg"].(string)
		return fmt.Errorf(err)
	}

	return nil
}
