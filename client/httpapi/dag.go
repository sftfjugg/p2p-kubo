package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
)

type HttpDagServ HttpApi

func (api *HttpDagServ) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	r, err := api.core().Block().Get(ctx, iface.IpldPath(c))
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	blk, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		return nil, err
	}

	return format.DefaultBlockDecoder.Decode(blk)
}

func (api *HttpDagServ) GetMany(ctx context.Context, cids []cid.Cid) <-chan *format.NodeOption {
	out := make(chan *format.NodeOption)
	wg := sync.WaitGroup{}
	wg.Add(len(cids))

	for _, c := range cids {
		// TODO: Consider limiting concurrency of this somehow
		go func() {
			defer wg.Done()
			n, err := api.Get(ctx, c)

			select {
			case out <- &format.NodeOption{Node: n, Err: err}:
			case <-ctx.Done():
			}
		}()
	}
	return out
}

func (api *HttpDagServ) Add(ctx context.Context, nd format.Node) error {
	c := nd.Cid()
	prefix := c.Prefix()
	format := cid.CodecToStr[prefix.Codec]
	if prefix.Version == 0 {
		format = "v0"
	}

	stat, err := api.core().Block().Put(ctx, bytes.NewReader(nd.RawData()),
		options.Block.Hash(prefix.MhType, prefix.MhLength), options.Block.Format(format))
	if err != nil {
		return err
	}
	if !stat.Path().Cid().Equals(c) {
		return fmt.Errorf("cids didn't match - local %s, remote %s", c.String(), stat.Path().Cid().String())
	}
	return nil
}

func (api *HttpDagServ) AddMany(ctx context.Context, nds []format.Node) error {
	for _, nd := range nds {
		// TODO: optimize
		if err := api.Add(ctx, nd); err != nil {
			return err
		}
	}
	return nil
}

func (api *HttpDagServ) Remove(ctx context.Context, c cid.Cid) error {
	return api.core().Block().Rm(ctx, iface.IpldPath(c)) //TODO: should we force rm?
}

func (api *HttpDagServ) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	for _, c := range cids {
		// TODO: optimize
		if err := api.Remove(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (api *HttpDagServ) core() *HttpApi {
	return (*HttpApi)(api)
}
