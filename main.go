package main

import (
	// "github.com/vrischmann/envconfig"
	"fmt"
	// "io/ioutil"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

func main() {
	// Get Token
	appId := "cli_a2f67c62dd22900c"
	appSecret := "4wKDQFOMKofqRtz3oBEfDhP5yPMPWz8G"

	token, err := genTenantAccessToken(appId, appSecret)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("token: ", token)

	// 从环境变量 USER_ID/CHAT_ID中获取 user_id/chat_id
	// chatId := "oc_a758de1e0c8bfdba639f342b3d79075a"
	// userId := "3e55b88e"
	chatId := os.Getenv("CHAT_ID")
	userId := os.Getenv("USER_ID")

	//通过chatID获取历史消息 message_id
	historyMessagesID, err := getHistoryMessage(chatId, token)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("messageID: ", historyMessagesID)


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

	//发送加急消息
	err = callPhone(historyMessagesID, userId, token)
	if err != nil {
		fmt.Println(err)
		return
	}

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
		// fmt.Println(err)
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
	var url string
	if len(pageToken) == 0 {
		url = "https://open.feishu.cn/open-apis/im/v1/messages?container_id=" + chatId + "&container_id_type=chat&page_size=50"
	} else {
		url = "https://open.feishu.cn/open-apis/im/v1/messages?container_id=" + chatId + "&container_id_type=chat&page_size=50&page_token=" + pageToken[0]
	}

	method := "GET"
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+authToken)
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
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
		return "", fmt.Errorf("faild1")
	}

	hasMore, _ := messData["has_more"].(bool)
	// fmt.Println("has more: ", hasMore)
	if hasMore {
		pageToken, _ := messData["page_token"].(string)
		getHistoryMessage(chatId, authToken, pageToken)
		return "", nil
	}

	messageItems, ok := messData["items"].([]interface{})
	if !ok {
		return "", fmt.Errorf("faild2")
	}

	latestItem, ok := messageItems[len(messageItems)-1].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("faild3")
	}

	messageId, ok := latestItem["message_id"].(string)
	if !ok {
		return "", fmt.Errorf("faild4")
	}
	return messageId, nil
}

func checkMessageStatus(messageId, authToken string) (bool, error) {
	// url := "https://open.feishu.cn/open-apis/im/v1/messages/om_2bb0eee978aa6f37e7b29f7e307508ca/read_users?user_id_type=user_id"
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
		return false, fmt.Errorf("faild1")
	}

	items, ok := messData["items"].([]interface{})
	if !ok {
		return false, fmt.Errorf("faild2")
	}

	if len(items) == 0 {
		// 消息未读
		return false, nil
	}

	return true, nil
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
	// fmt.Println("body: ", jsonData)
	if jsonData["code"].(float64) != 0 {
		err, _ := jsonData["msg"].(string)
		return fmt.Errorf(err)
	}
	
	return nil
}
