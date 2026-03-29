package scene

import "testing"

func TestLooksLikeSwitchCommand(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"切换场景到终端",
		"切到终端，然后 ls -la",
		"switch scene to terminal",
		"Set scene to terminal",
		"シーンをターミナルに切り替え",
	} {
		if !LooksLikeSwitchCommand(input) {
			t.Fatalf("LooksLikeSwitchCommand(%q) = false, want true", input)
		}
	}
}

func TestLooksLikeSwitchCommandRejectsNormalDictation(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"用 grep 查一下 error log",
		"terminal 里面有个文件",
		"今日は terminal を開いた",
	} {
		if LooksLikeSwitchCommand(input) {
			t.Fatalf("LooksLikeSwitchCommand(%q) = true, want false", input)
		}
	}
}
