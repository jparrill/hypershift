package util

import (
	"reflect"
	"testing"
)

func TestRemoveDuplicatesFromList(t *testing.T) {
	type args struct {
		addresses []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Given a slice with duplicated entries, it should return the slice without them",
			args: args{
				addresses: []string{"abc", "abc", "abcd", "abcde"},
			},
			want: []string{"abc", "abcd", "abcde"},
		},
		{
			name: "Given a slice with all entries duplicated, it should return the slice with just one entry",
			args: args{
				addresses: []string{"abc", "abc", "abc", "abc"},
			},
			want: []string{"abc"},
		},
		{
			name: "Given a slice without duplicated entries, it should return the slice with the same content",
			args: args{
				addresses: []string{"abc", "abcd", "abcde"},
			},
			want: []string{"abc", "abcd", "abcde"},
		},
		{
			name: "Given an empty slice, it should return the slice with the same content",
			args: args{
				addresses: []string{},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveDuplicatesFromList(tt.args.addresses); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoveDuplicatesFromList() = %v, want %v", got, tt.want)
			}
		})
	}
}
