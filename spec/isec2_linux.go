package spec

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/Songmu/retry"
)

var uuidFiles = [2]string{
	"/sys/hypervisor/uuid",
	"/sys/devices/virtual/dmi/id/product_uuid",
}

// If the OS is Linux, check /sys/hypervisor/uuid and /sys/devices/virtual/dmi/id/product_uuid files first. If UUID seems to be EC2-ish, call the metadata API (up to 3 times).
// ref. https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/identify_ec2_instances.html
func isEC2() bool {
	looksLikeEC2 := false
	for _, u := range uuidFiles {
		data, err := ioutil.ReadFile(u)
		if err != nil {
			continue
		}
		if isEC2UUID(string(data)) {
			looksLikeEC2 = true
			break
		}
	}
	if !looksLikeEC2 {
		return false
	}

	res := false
	cl := httpCli()
	err := retry.Retry(3, 2*time.Second, func() error {
		// '/ami-id` is probably an AWS specific URL
		resp, err := cl.Get(ec2BaseURL.String() + "/ami-id")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		res = resp.StatusCode == 200
		return nil
	})

	if err == nil {
		return res
	}

	return false
}

func isEC2UUID(uuid string) bool {
	conds := func(uuid string) bool {
		if strings.HasPrefix(uuid, "ec2") || strings.HasPrefix(uuid, "EC2") {
			return true
		}
		return false
	}

	if conds(uuid) {
		return true
	}

	// Check as littele endian.
	// see. https://docs.aws.amazon.com/ja_jp/AWSEC2/latest/UserGuide/identify_ec2_instances.html
	fields := strings.Split(uuid, "-")
	src := []byte(fields[0]) // UUID field: time_low(uint32)

	dst := make([]byte, hex.DecodedLen(len(src)))
	hex.Decode(dst, src)

	r := bytes.NewReader(dst)
	var data uint32
	binary.Read(r, binary.LittleEndian, &data)

	if conds(fmt.Sprintf("%x", data)) {
		return true
	}

	return false
}
