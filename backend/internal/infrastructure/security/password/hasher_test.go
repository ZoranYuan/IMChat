package password

import "testing"

func TestHasher(t *testing.T) {
	hasher := NewHasher()
	hashed, err := hasher.Hash("正确密码")
	if err != nil {
		t.Fatalf("生成密码摘要失败：%v", err)
	}
	if hashed == "正确密码" {
		t.Fatal("密码摘要不应等于明文")
	}
	if !hasher.Verify("正确密码", hashed) {
		t.Fatal("正确密码校验失败")
	}
	if hasher.Verify("错误密码", hashed) {
		t.Fatal("错误密码不应通过校验")
	}
}
