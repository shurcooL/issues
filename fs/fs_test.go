package fs

import (
	"reflect"
	"testing"

	"github.com/shurcooL/issues"
	"github.com/shurcooL/users"
)

func TestToggleReaction(t *testing.T) {
	c := comment{
		Reactions: []reaction{
			{EmojiID: issues.EmojiID("bar"), Authors: []userSpec{{ID: 1}, {ID: 2}}},
			{EmojiID: issues.EmojiID("baz"), Authors: []userSpec{{ID: 3}}},
		},
	}

	toggleReaction(&c, users.UserSpec{ID: 1}, issues.EmojiID("foo"))
	toggleReaction(&c, users.UserSpec{ID: 1}, issues.EmojiID("bar"))
	toggleReaction(&c, users.UserSpec{ID: 1}, issues.EmojiID("baz"))
	toggleReaction(&c, users.UserSpec{ID: 2}, issues.EmojiID("bar"))

	want := comment{
		Reactions: []reaction{
			{EmojiID: issues.EmojiID("baz"), Authors: []userSpec{{ID: 3}, {ID: 1}}},
			{EmojiID: issues.EmojiID("foo"), Authors: []userSpec{{ID: 1}}},
		},
	}

	if got := c; !reflect.DeepEqual(got, want) {
		t.Errorf("\ngot  %+v\nwant %+v", got.Reactions, want.Reactions)
	}
}
