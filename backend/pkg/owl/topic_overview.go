package owl

import (
	"sort"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

// TopicOverview is all information we get when listing Kafka topics
type TopicOverview struct {
	TopicName         string `json:"topicName"`
	IsInternal        bool   `json:"isInternal"`
	PartitionCount    int    `json:"partitionCount"`
	ReplicationFactor int    `json:"replicationFactor"`
	CleanupPolicy     string `json:"cleanupPolicy"`
	LogDirSize        int64  `json:"logDirSize"`

	// What actions the logged in user is allowed to run on this topic
	AllowedActions []string `json:"allowedActions"`
}

// GetTopicsOverview returns a TopicOverview for all Kafka Topics
func (s *Service) GetTopicsOverview() ([]*TopicOverview, error) {
	topics, err := s.kafkaSvc.ListTopics()
	if err != nil {
		return nil, err
	}

	// 3. Get log dir sizes for each topic
	sizeByTopic, err := s.logDirSizeByTopic()
	if err != nil {
		return nil, err
	}

	// 3. Create config resources request objects for all topics
	topicNames := make([]string, len(topics))
	for i, topic := range topics {
		if topic.Err != sarama.ErrNoError {
			s.logger.Error("failed to get topic metadata while listing topics",
				zap.String("topic_name", topic.Name),
				zap.Error(topic.Err))
			return nil, topic.Err
		}

		topicNames[i] = topic.Name
	}

	configs, err := s.GetTopicsConfigs(topicNames, []string{"cleanup.policy"})
	if err != nil {
		return nil, err
	}

	// x. Merge information from all requests and construct the TopicOverview object
	res := make([]*TopicOverview, len(topicNames))
	for i, topic := range topics {
		size := int64(-1)
		if value, ok := sizeByTopic[topic.Name]; ok {
			size = value
		}

		policy := "unknown"
		if val, ok := configs[topic.Name]; ok {
			entry := val.GetConfigEntryByName("cleanup.policy")
			if entry != nil {
				policy = entry.Value
			}
		}

		res[i] = &TopicOverview{
			TopicName:         topic.Name,
			IsInternal:        topic.IsInternal,
			PartitionCount:    len(topic.Partitions),
			ReplicationFactor: len(topic.Partitions[0].Replicas),
			CleanupPolicy:     policy,
			LogDirSize:        size,
		}
	}

	// 5. Return map as array which is sorted by topic name
	sort.Slice(res, func(i, j int) bool {
		return res[i].TopicName < res[j].TopicName
	})

	return res, nil
}
