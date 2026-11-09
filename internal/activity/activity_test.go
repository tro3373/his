package activity

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var jst = time.FixedZone("JST", 9*3600)

func at(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, jst) }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAppendRead(t *testing.T) {
	a := Entry{At: at(3, 10, 0), Cwd: "/w/a", Branch: "main"}
	b := Entry{At: at(3, 11, 0), Cwd: "/w/b"}
	c := Entry{At: at(4, 0, 30), Cwd: "/w/c", Branch: "x"}
	tests := []struct {
		name    string
		entries []Entry
		since   time.Time
		want    []Entry
	}{
		{name: "追記した場合_そのまま読み戻せること", entries: []Entry{a, b, c}, since: at(3, 0, 0), want: []Entry{a, b, c}},
		{name: "sinceより前の場合_読まないこと", entries: []Entry{a, b, c}, since: at(3, 10, 30), want: []Entry{b, c}},
		{name: "sinceの日より前のファイルの場合_読まないこと", entries: []Entry{a, c}, since: at(4, 0, 0), want: []Entry{c}},
		{name: "cwdに改行を含む場合_書かないこと", entries: []Entry{{At: at(3, 9, 0), Cwd: "/w\nx"}}, since: at(3, 0, 0), want: nil},
		{name: "cwdにタブを含む場合_書かないこと", entries: []Entry{{At: at(3, 9, 0), Cwd: "/w\tx"}}, since: at(3, 0, 0), want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "act")
			for _, e := range tt.entries {
				if err := Append(dir, e); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Read(dir, tt.since)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Read() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppend_perm(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "act")
	if err := Append(dir, Entry{At: at(3, 10, 0), Cwd: "/w"}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path string
		want os.FileMode
	}{
		{name: "ディレクトリの場合_0700であること", path: dir, want: 0o700},
		{name: "ファイルの場合_0600であること", path: filepath.Join(dir, "20261103.tsv"), want: 0o600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := os.Stat(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if got := st.Mode().Perm(); got != tt.want {
				t.Errorf("perm = %o, want %o", got, tt.want)
			}
		})
	}
}

func TestRead(t *testing.T) {
	tests := []struct {
		name    string
		content string
		noDir   bool
		want    []Entry
	}{
		{name: "不正な行の場合_飛ばして読むこと", content: "garbage\n1\t/a\n\nx\t/a\tb\n" + "1793667600\t/ok\tbr\n\t\t\n1793667600\t\tbr\n", want: []Entry{{At: time.Unix(1793667600, 0).In(jst), Cwd: "/ok", Branch: "br"}}},
		{name: "列が多すぎる場合_読まないこと", content: "1793667600\t/a\tb\textra\n", want: nil},
		{name: "dirが無い場合_空を返しエラーにしないこと", noDir: true, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "activity")
			if !tt.noDir {
				writeFile(t, filepath.Join(dir, "20261103.tsv"), tt.content)
			}
			got, err := Read(dir, at(3, 0, 0))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Read() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrune(t *testing.T) {
	tests := []struct {
		name   string
		files  []string
		before time.Time
		want   []string
	}{
		{
			name:   "基準日より前の場合_消し基準日以降は残すこと",
			files:  []string{"20261101.tsv", "20261102.tsv", "20261103.tsv", "20261104.tsv"},
			before: at(3, 15, 0),
			want:   []string{"20261103.tsv", "20261104.tsv"},
		},
		{
			name:   "日付名でないファイルの場合_触らないこと",
			files:  []string{"20261101.tsv", "notes.tsv", "20261101.txt", "20261101", "20261301.tsv"},
			before: at(3, 0, 0),
			want:   []string{"20261101", "20261101.txt", "20261301.tsv", "notes.tsv"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				writeFile(t, filepath.Join(dir, f), "")
			}
			if err := Prune(dir, tt.before); err != nil {
				t.Fatal(err)
			}
			ents, _ := os.ReadDir(dir)
			var got []string
			for _, e := range ents {
				got = append(got, e.Name())
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("files = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrune_missingDir(t *testing.T) {
	if err := Prune(filepath.Join(t.TempDir(), "none"), at(3, 0, 0)); err != nil {
		t.Errorf("Prune() = %v, want nil", err)
	}
}
