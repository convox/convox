package router

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type StorageDynamo struct {
	ddb    *dynamodb.DynamoDB
	hosts  string
	routes string
}

func NewStorageDynamo(hosts, routes string) (*StorageDynamo, error) {
	fmt.Printf("ns=storage.dynamo at=new hosts=%s routes=%s\n", hosts, routes)

	s, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	d := &StorageDynamo{
		ddb:    dynamodb.New(s),
		hosts:  hosts,
		routes: routes,
	}

	return d, nil
}

func (s *StorageDynamo) HostList() ([]string, error) {
	// fmt.Printf("ns=storage.dynamo at=host.list\n")

	ctx := context.Background()

	req := &dynamodb.ScanInput{
		ProjectionExpression: aws.String("host"),
		TableName:            aws.String(s.hosts),
	}

	p := request.Pagination{
		NewRequest: func() (*request.Request, error) {
			r, _ := s.ddb.ScanRequest(req)
			r.SetContext(ctx)
			return r, nil
		},
	}

	hosts := []string{}

	for p.Next() {
		page := p.Page().(*dynamodb.ScanOutput)

		for _, item := range page.Items {
			if host := item["host"].S; host != nil {
				hosts = append(hosts, *host)
			}
		}
	}

	if err := p.Err(); err != nil {
		return nil, err
	}

	return hosts, nil
}

func (s *StorageDynamo) IdleGet(target string) (bool, error) {
	return false, nil
}

func (s *StorageDynamo) IdleSet(target string, idle bool) error {
	fmt.Printf("ns=storage.dynamo at=idle.get target=%q idle=%t\n", target, idle)

	return nil
}

func (s *StorageDynamo) RequestBegin(target string) error {
	fmt.Printf("ns=storage.dynamo at=request.begin target=%q\n", target)

	return nil
}

func (s *StorageDynamo) RequestEnd(target string) error {
	fmt.Printf("ns=storage.dynamo at=request.end target=%q\n", target)

	return nil
}

func (s *StorageDynamo) Stale(cutoff time.Time) ([]string, error) {
	fmt.Printf("ns=storage.dynamo at=stale cutoff=%s\n", cutoff)

	return []string{}, nil
}

func (s *StorageDynamo) TargetAdd(host, target string, idles bool) error {
	fmt.Printf("ns=storage.dynamo at=target.add host=%q target=%q\n", host, target)

	_, err := s.ddb.PutItem(&dynamodb.PutItemInput{
		Item:      map[string]*dynamodb.AttributeValue{"host": {S: aws.String(host)}},
		TableName: aws.String(s.hosts),
	})
	if err != nil {
		return err
	}

	_, err = s.ddb.PutItem(&dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"host":   {S: aws.String(host)},
			"target": {S: aws.String(target)},
		},
		TableName: aws.String(s.routes),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *StorageDynamo) TargetList(host string) ([]string, error) {
	// fmt.Printf("ns=storage.dynamo at=target.list\n")

	res, err := s.ddb.Query(&dynamodb.QueryInput{
		ExpressionAttributeNames:  map[string]*string{"#host": aws.String("host")},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{":host": {S: aws.String(host)}},
		KeyConditionExpression:    aws.String("#host = :host"),
		TableName:                 aws.String(s.routes),
	})
	if err != nil {
		return nil, err
	}

	ts := []string{}

	for _, item := range res.Items {
		if t := item["target"].S; t != nil {
			ts = append(ts, *t)
		}
	}

	return ts, nil
}

func (s *StorageDynamo) TargetRemove(host, target string) error {
	fmt.Printf("ns=storage.dynamo at=target.remove host=%q target=%q\n", host, target)

	_, err := s.ddb.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"host":   {S: aws.String(host)},
			"target": {S: aws.String(target)},
		},
		TableName: aws.String(s.routes),
	})
	if err != nil {
		return err
	}

	ts, err := s.TargetList(host)
	if err != nil {
		return err
	}

	if len(ts) == 0 {
		_, err := s.ddb.DeleteItem(&dynamodb.DeleteItemInput{
			Key: map[string]*dynamodb.AttributeValue{
				"host": {S: aws.String(host)},
			},
			TableName: aws.String(s.hosts),
		})
		if err != nil {
			return err
		}
	}

	return nil
}
