// 聊天的数据库操作
package database

import (
	"database/sql"
	"fmt"

	"go-seckill/models"
)

// FindOrCreateConversation 找或建会话
// trick：强制把ID小的放user1，这样同一对人不会创建出两个会话
func FindOrCreateConversation(u1ID int, u1Role string, u2ID int, u2Role string) (*models.Conversation, error) {
	// 排序保证：不管谁先发消息，查出来的user1和user2顺序一样
	if u1ID > u2ID || (u1ID == u2ID && u1Role > u2Role) {
		u1ID, u2ID = u2ID, u1ID
		u1Role, u2Role = u2Role, u1Role
	}

	var c models.Conversation
	err := DB.QueryRow(
		`SELECT conversation_id, user1_id, user1_role, user2_id, user2_role, last_message, last_message_at, created_at, updated_at
		 FROM conversations WHERE user1_id=? AND user1_role=? AND user2_id=? AND user2_role=?`,
		u1ID, u1Role, u2ID, u2Role,
	).Scan(&c.ConversationID, &c.User1ID, &c.User1Role, &c.User2ID, &c.User2Role,
		&c.LastMessage, &c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt)

	if err == nil {
		return &c, nil // 已经有会话了
	}
	if err != sql.ErrNoRows {
		return nil, err // 出错了
	}

	// 没找到，建一个新会话
	result, err := DB.Exec(
		`INSERT INTO conversations (user1_id, user1_role, user2_id, user2_role) VALUES (?, ?, ?, ?)`,
		u1ID, u1Role, u2ID, u2Role,
	)
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}
	id, _ := result.LastInsertId()
	c = models.Conversation{
		ConversationID: int(id),
		User1ID:        u1ID,
		User1Role:      u1Role,
		User2ID:        u2ID,
		User2Role:      u2Role,
	}
	return &c, nil
}

// SaveMessage 存消息，同时更新会话的最后一条消息
func SaveMessage(convID, senderID int, senderRole string, receiverID int, receiverRole, content string) (*models.ChatMessage, error) {
	result, err := DB.Exec(
		`INSERT INTO chat_messages (conversation_id, sender_id, sender_role, receiver_id, receiver_role, content)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		convID, senderID, senderRole, receiverID, receiverRole, content,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()

	// 顺手更新会话的最后消息，这样会话列表能显示最新消息
	DB.Exec(`UPDATE conversations SET last_message=?, last_message_at=NOW(), updated_at=NOW() WHERE conversation_id=?`,
		content, convID)

	return &models.ChatMessage{
		MessageID:      int(id),
		ConversationID: convID,
		SenderID:       senderID,
		SenderRole:     senderRole,
		ReceiverID:     receiverID,
		ReceiverRole:   receiverRole,
		Content:        content,
	}, nil
}

// GetConversationMessages 拉历史消息，带分页
func GetConversationMessages(convID, limit, offset int) ([]models.ChatMessage, error) {
	rows, err := DB.Query(
		`SELECT message_id, conversation_id, sender_id, sender_role, receiver_id, receiver_role, content, read_at, created_at
		 FROM chat_messages WHERE conversation_id = ? ORDER BY created_at ASC LIMIT ? OFFSET ?`,
		convID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.ChatMessage
	for rows.Next() {
		var m models.ChatMessage
		rows.Scan(&m.MessageID, &m.ConversationID, &m.SenderID, &m.SenderRole,
			&m.ReceiverID, &m.ReceiverRole, &m.Content, &m.ReadAt, &m.CreatedAt)
		messages = append(messages, m)
	}
	return messages, nil
}

// GetUserConversations 拿这个用户的所有会话，按最新消息时间倒序
func GetUserConversations(userID int, role string) ([]models.Conversation, error) {
	rows, err := DB.Query(
		`SELECT conversation_id, user1_id, user1_role, user2_id, user2_role, last_message, last_message_at, created_at, updated_at
		 FROM conversations
		 WHERE (user1_id=? AND user1_role=?) OR (user2_id=? AND user2_role=?)
		 ORDER BY last_message_at DESC`,
		userID, role, userID, role,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []models.Conversation
	for rows.Next() {
		var c models.Conversation
		rows.Scan(&c.ConversationID, &c.User1ID, &c.User1Role, &c.User2ID, &c.User2Role,
			&c.LastMessage, &c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt)
		convs = append(convs, c)
	}
	return convs, nil
}

// MarkMessagesRead 标已读：把发给这个用户的未读消息全部标成已读
func MarkMessagesRead(receiverID int, receiverRole string, conversationID int) error {
	_, err := DB.Exec(
		`UPDATE chat_messages SET read_at=NOW() WHERE receiver_id=? AND receiver_role=? AND conversation_id=? AND read_at IS NULL`,
		receiverID, receiverRole, conversationID,
	)
	return err
}

// GetUnreadCount 数未读消息有几条
func GetUnreadCount(receiverID int, receiverRole string, conversationID int) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM chat_messages WHERE receiver_id=? AND receiver_role=? AND conversation_id=? AND read_at IS NULL`,
		receiverID, receiverRole, conversationID,
	).Scan(&count)
	return count, err
}
