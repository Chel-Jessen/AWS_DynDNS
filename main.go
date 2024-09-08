package main

import (
	"encoding/json"
	"flag"
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

func updateRecord(accessKey string, secretKey string, hostedZoneID string, domainName string, subdomain string, region string) error {
	var ip = getExternalIP()
	println("Current IP: ", ip)
	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region), // Change to your desired region
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
	var accessKey string
	var secretKey string
	var hostedZoneID string
	var domainName string
	var subdomain string
	var region string
	var secs int

	flag.StringVar(&accessKey, "accessKey", "", "AWS Access Key")
	flag.StringVar(&accessKey, "a", "", "AWS Access Key")
	flag.StringVar(&secretKey, "secretKey", "", "AWS Secret Key")
	flag.StringVar(&secretKey, "s", "", "AWS Secret Key")
	flag.StringVar(&hostedZoneID, "hostedZoneID", "", "Hosted Zone ID")
	flag.StringVar(&hostedZoneID, "hz", "", "Hosted Zone ID")
	flag.StringVar(&domainName, "domain", "", "Domain Name")
	flag.StringVar(&subdomain, "subdomain", "", "Subdomain")
	flag.StringVar(&region, "region", "us-east-1", "Region")
	flag.IntVar(&secs, "sec", 300, "Subdomain")

	flag.Parse()

	for {
		err := updateRecord(accessKey, secretKey, hostedZoneID, domainName, subdomain, region)
		if err != nil {
			println(err)
			return
		}
		time.Sleep(time.Second * time.Duration(secs))
	}
}
