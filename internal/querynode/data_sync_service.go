package querynode

import (
	"context"

	"go.uber.org/zap"

	"github.com/zilliztech/milvus-distributed/internal/log"
	"github.com/zilliztech/milvus-distributed/internal/msgstream"
	"github.com/zilliztech/milvus-distributed/internal/proto/internalpb2"
	"github.com/zilliztech/milvus-distributed/internal/util/flowgraph"
)

type dataSyncService struct {
	ctx context.Context
	fg  *flowgraph.TimeTickedFlowGraph

	dmStream  msgstream.MsgStream
	msFactory msgstream.Factory

	replica ReplicaInterface
}

func newDataSyncService(ctx context.Context, replica ReplicaInterface, factory msgstream.Factory) *dataSyncService {
	service := &dataSyncService{
		ctx:       ctx,
		fg:        nil,
		replica:   replica,
		msFactory: factory,
	}

	service.initNodes()
	return service
}

func (dsService *dataSyncService) start() {
	dsService.fg.Start()
}

func (dsService *dataSyncService) close() {
	if dsService.fg != nil {
		dsService.fg.Close()
	}
}

func (dsService *dataSyncService) initNodes() {
	// TODO: add delete pipeline support

	dsService.fg = flowgraph.NewTimeTickedFlowGraph(dsService.ctx)

	var dmStreamNode node = dsService.newDmInputNode(dsService.ctx)

	var filterDmNode node = newFilteredDmNode(dsService.replica)

	var insertNode node = newInsertNode(dsService.replica)
	var serviceTimeNode node = newServiceTimeNode(dsService.ctx, dsService.replica, dsService.msFactory)

	dsService.fg.AddNode(dmStreamNode)

	dsService.fg.AddNode(filterDmNode)

	dsService.fg.AddNode(insertNode)
	dsService.fg.AddNode(serviceTimeNode)

	// dmStreamNode
	var err = dsService.fg.SetEdges(dmStreamNode.Name(),
		[]string{},
		[]string{filterDmNode.Name()},
	)
	if err != nil {
		log.Error("set edges failed in node:", zap.String("node name", dmStreamNode.Name()))
	}

	// filterDmNode
	err = dsService.fg.SetEdges(filterDmNode.Name(),
		[]string{dmStreamNode.Name()},
		[]string{insertNode.Name()},
	)
	if err != nil {
		log.Error("set edges failed in node:", zap.String("node name", filterDmNode.Name()))
	}

	// insertNode
	err = dsService.fg.SetEdges(insertNode.Name(),
		[]string{filterDmNode.Name()},
		[]string{serviceTimeNode.Name()},
	)
	if err != nil {
		log.Error("set edges failed in node:", zap.String("node name", insertNode.Name()))
	}

	// serviceTimeNode
	err = dsService.fg.SetEdges(serviceTimeNode.Name(),
		[]string{insertNode.Name()},
		[]string{},
	)
	if err != nil {
		log.Error("set edges failed in node:", zap.String("node name", serviceTimeNode.Name()))
	}
}

func (dsService *dataSyncService) seekSegment(position *internalpb2.MsgPosition) error {
	err := dsService.dmStream.Seek(position)
	if err != nil {
		return err
	}
	return nil
}
