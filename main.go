package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"io"
	"net/http"
	"time"
)

func getExternalIP() string {
	type IP struct {
		Ip string `json:"origin"`
	}
	resp, _ := http.Get("http://httpbin.org/ip")
	body, _ := io.ReadAll(resp.Body)
	var ip IP
	_ = json.Unmarshal(body, &ip)
	return ip.Ip
}

func updateRecord(accessKey string, secretKey string, hostedZoneID string, domainName string, subdomain string) error {
	var ip = getExternalIP()
	println("Current IP: ", ip)
	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"), // Change to your desired region
		Credentials: creds,
	})
	if err != nil {
		return err
	}

	svc := route53.New(sess)

	// Find the existing record
	recordName := subdomain + "." + domainName + "."
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(hostedZoneID),
		StartRecordName: aws.String(recordName),
		StartRecordType: aws.String(route53.RRTypeA),
		MaxItems:        aws.String("1"),
	}

	output, err := svc.ListResourceRecordSets(input)
	if err != nil {
		return err
	}

	if len(output.ResourceRecordSets) == 0 || *output.ResourceRecordSets[0].Name != recordName {
		return fmt.Errorf("could not find A record for subdomain %s", subdomain)
	}
	var oldIP string

	for _, rr := range output.ResourceRecordSets {
		if *rr.Name == recordName && *rr.Type == "A" && len(rr.ResourceRecords) > 0 {
			oldIP = *rr.ResourceRecords[0].Value
			break
		}
	}
	if oldIP == ip {
		println("IP is up to date")
		return nil
	}
	// Update the record
	record := output.ResourceRecordSets[0]
	record.TTL = aws.Int64(300)
	record.ResourceRecords = []*route53.ResourceRecord{
		{
			Value: aws.String(ip),
		},
	}

	changeInput := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action:            aws.String(route53.ChangeActionUpsert),
					ResourceRecordSet: record,
				},
			},
			Comment: aws.String("Update record"),
		},
	}

	_, err = svc.ChangeResourceRecordSets(changeInput)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	// Replace with your own values
	var accessKey = "xxxxxxxxxxxxxx"
	var secretKey = "xxxxxxxxxxxxxxxxxxxxx"
	var hostedZoneID = "xxxxxxxxxxxxxxx"
	var domainName = "example.net"
	var subdomain = "subexample"

	for {
		err := updateRecord(accessKey, secretKey, hostedZoneID, domainName, subdomain)
		if err != nil {
			println(err)
			return
		}
		time.Sleep(time.Second * 3600)
	}
}
