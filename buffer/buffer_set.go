// bufferSet
// - Holds data and information of a file
// - Stores information necessary for editing the file, such as cursor position
//   These are kept together as a set.

package buffer

import "github.com/ge-editor/editorview/file"

func newBufferSet(filePath string) *bufferSet {
	file := file.NewFile(filePath)
	return &bufferSet{
		File:  file,
		metas: make([]*Meta, 0, 2),
	}
}

type bufferSet struct {
	*file.File
	metas []*Meta
}

func (bs *bufferSet) GetMetas() []*Meta {
	return bs.metas
}

func (bs *bufferSet) PushMeta(m *Meta) {
	bs.metas = append(bs.metas, m)
}

func (bs *bufferSet) PopMeta() *Meta {
	if len(bs.metas) > 0 {
		lastMeta := bs.metas[len(bs.metas)-1]
		bs.metas = bs.metas[:len(bs.metas)-1]
		return lastMeta
	}
	return newMeta()
}
