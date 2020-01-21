package apihandler

import (
	pb "github.com/transavro/TileService/proto"
	"context"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/proto"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"strings"
	"sync"
)

type Server struct {
	TileCollection *mongo.Collection
	RedisConnection  *redis.Client
}

func(s *Server) checkInRedis(redisKey string) bool{
	if s.RedisConnection.Exists(redisKey).Val() == 1 {
		return  true
	}else {
		return  false
	}
}


func(s *Server) CloudwalkerPrimePages(ctx context.Context, request *pb.PrimePagesRequest)(*pb.CloudwalkerSchedule, error){
	log.Println("Prime hit")
		redisKey := fmt.Sprintf("%s:%s:cloudwalkerPrimePages", strings.ToLower(request.GetVendor()), strings.ToLower(request.GetBrand()))
		if s.checkInRedis(redisKey) {
			result, err := s.RedisConnection.SMembers(redisKey).Result()
			if err != nil {
				return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to get Result from Cache ", err))
			}
			var schedule pb.CloudwalkerSchedule
			for _, value := range result {
				var page pb.Page
				//Important lesson, challenge was to convert interface{} to byte. used  ([]byte(k.(string)))
				err = proto.Unmarshal([]byte(value), &page)
				if err != nil {
					return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to unMarshal result ", err))
				}
				schedule.Pages = append(schedule.Pages, &page)
			}
			return &schedule, nil
		}else{
			return nil, status.Error(codes.DataLoss, fmt.Sprintf("Data not found on cache "))
		}
}

func (s *Server) GetPage(ctx context.Context, request *pb.PageRequest)(*pb.PageResponse, error)  {
	log.Println("Get page")
	redisKey := fmt.Sprintf("%s:%s:%s", strings.ToLower(request.GetVendor()), strings.ToLower(request.GetBrand()), strings.ToLower(request.GetPageName()))
	if s.checkInRedis(redisKey) {
		result, err := s.RedisConnection.SMembers(redisKey).Result()
		if err != nil {
			return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to get Result from Cache ", err))
		}
		var response pb.PageResponse
		err = proto.Unmarshal([]byte(result[0]), &response)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to unMarshal result ", err))
		}
		return &response, nil
	}else {
		return nil, status.Error(codes.DataLoss, fmt.Sprintf("Data not found on cache "))
	}
}

func (s *Server) GetCarousel(ctx context.Context, request *pb.CarouselRequest) (*pb.CarouselResponse, error) {
	log.Println("Carosuel Hit.")
	redisKey := fmt.Sprintf("%s:%s:%s:carousel", strings.ToLower(request.GetVendor()), strings.ToLower(request.GetBrand()), strings.ToLower(request.GetPageName()))
	if s.checkInRedis(redisKey) {
		result, err := s.RedisConnection.SMembers(redisKey).Result()
		if err != nil {
			return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to get Result from Cache ", err))
		}
		var carouselResponse pb.CarouselResponse
		carouselResponse.CarouselBaseUrl = "http://cloudwalker-assets-prod.s3.ap-south-1.amazonaws.com/images/tiles/"
		for _, value := range result {
			var response pb.Carousel
			err = proto.Unmarshal([]byte(value), &response)
			if err != nil {
				continue
				//return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to unMarshal result ", err))
			}
			carouselResponse.Carousel = append(carouselResponse.Carousel, &response)
		}
		return &carouselResponse, nil
	}else {
		return nil, status.Error(codes.DataLoss, fmt.Sprintf("Data not found on cache "))
	}
}

func (s *Server) GetRow(ctx context.Context,  request *pb.RowRequest) (*pb.RowResponse, error)  {
	log.Println("row hit")
	redisKey := fmt.Sprintf("%s:%s:%s:%s:%s", strings.ToLower(request.GetVendor()), strings.ToLower(request.GetBrand()), strings.ToLower(request.GetPageName()), strings.ToLower(request.GetRowName()), strings.ToLower(request.GetRowType()))
	if s.checkInRedis(redisKey) {
		var interResp pb.InterRowResponse
		result, err := s.RedisConnection.SMembers(redisKey).Result()
		if err != nil {
			return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to get Result from Cache ", err))
		}

		err = proto.Unmarshal([]byte(result[0]), &interResp)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to unMarshal result ", err))
		}
		var response pb.RowResponse
		response.RowName = interResp.RowName
		response.RowLayout = interResp.RowLayout
		response.ContentBaseUrl = interResp.ContentBaseUrl

		if s.checkInRedis(interResp.ContentId) {
			if request.RowType == strings.ToLower(pb.RowType_Editorial.String()) {
				result, err := s.RedisConnection.ZRange(interResp.GetContentId(), 0  , -1).Result()
				if err != nil {
					return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to get Result from Cache ", err))
				}
				for _ , v := range result {
					var contentTile pb.ContentTile
					err = proto.Unmarshal([]byte(v), &contentTile)
					if err != nil {
						continue
					}
					response.ContentTiles = append(response.ContentTiles, &contentTile)
				}
			}else {
				var result []string
				if interResp.Shuffle == true {
					result, err = s.RedisConnection.SRandMemberN(interResp.ContentId, 15).Result()
					if err != nil {
						return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to get Result from Cache ", err))
					}
				}else {
					result, err = s.RedisConnection.SMembers(interResp.ContentId).Result()
					if err != nil {
						return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to get Result from Cache ", err))
					}
				}

				for i , v := range result {
					if i < 15 {
						var contentTile pb.ContentTile
						err = proto.Unmarshal([]byte(v), &contentTile)
						if err != nil {
							continue
						}
						response.ContentTiles = append(response.ContentTiles, &contentTile)
					}
				}
			}
		}
		return &response, nil

	}else {
		return nil, status.Error(codes.DataLoss, fmt.Sprintf("Data not found on cache "))
	}
}

func(s *Server) GetContent(request *pb.RowRequest, stream pb.TileService_GetContentServer) error {
	log.Println("content hit.")
	redisKey := fmt.Sprintf("%s:%s:%s:%s:%s:content", strings.ToLower(request.GetVendor()), strings.ToLower(request.GetBrand()), strings.ToLower(request.GetPageName()), strings.ToLower(request.GetRowName()), strings.ToLower(request.GetRowType()))

	if s.checkInRedis(redisKey) {
		if request.RowType == strings.ToLower(pb.RowType_Editorial.String()){
			result, err := s.RedisConnection.ZRange(redisKey, 0  , -1).Result()
			if err != nil {
				return status.Error(codes.Unavailable, fmt.Sprintf("Failed to get Result from Cache ", err))
			}
			for _ , v := range result {
				var contentTile pb.ContentTile
				err = proto.Unmarshal([]byte(v), &contentTile)
				if err != nil {
					continue
				}
				err = stream.Send(&contentTile)
				if err != nil {
					return status.Error(codes.Internal, fmt.Sprintf("Stream sending error ", err))
				}
			}
		}else {
			var nextCursor uint64
			var wg sync.WaitGroup
			for {
				//result, serverCursor, err := s.RedisConnection.SScan(req.RowId, nextCursor, "", chunkDataCount).Result()
				result, serverCursor, err := s.RedisConnection.SScan(redisKey, nextCursor, "", 300).Result()
				if err != nil {
					return status.Error(codes.Internal, fmt.Sprintf("Cache parsing error ", err))
				}
				nextCursor = serverCursor
				wg.Add(1)

				go func(redisConn *redis.Client) error {
					for _, k := range result {
						//Important lesson, challenge was to convert interface{} to byte. used  ([]byte(k.(string)))
						var movieTile pb.ContentTile
						err = proto.Unmarshal([]byte(k), &movieTile)
						if err != nil {
							return status.Error(codes.Internal, fmt.Sprintf("Failed to unMarshal result ", err))
						}
						err = stream.Send(&movieTile)
						if err != nil {
							return status.Error(codes.Internal, fmt.Sprintf("Stream sending error ", err))
						}
					}
					wg.Done()
					return nil
				}(s.RedisConnection)

				wg.Wait()
				if serverCursor == 0 {
					break
				}
			}
		}
		return nil;
	}else {
		return status.Error(codes.DataLoss, fmt.Sprintf("Data not found on cache "))
	}
}





//TODO GOLD
//log.Println("getRows 1   "+req.RowId)
//var rowResp pb.RowResponse
//
//infoSet := strings.Split(req.RowId, "-")
//
//rowResp.RowName = strings.Replace(infoSet[1], "_", " ", -1)
//
//result, err := s.RedisConnection.SMembers(fmt.Sprintf("%s-%s-rowLayout",infoSet[0], infoSet[1])).Result()
//if err != nil {
//	return nil, err
//}
//rowResp.RowLayout = result[0]
//
//baseUrlResult, err := s.RedisConnection.SMembers(fmt.Sprintf("%s-row-baseurl", infoSet[0])).Result()
//if err != nil {
//	log.Println("getRows 4")
//	return nil, err
//}
//rowResp.ContentBaseUrl = baseUrlResult[0]
//
//
//var nextCursor uint64
//var wg sync.WaitGroup
//for {
//	//result, serverCursor, err := s.RedisConnection.SScan(req.RowId, nextCursor, "", chunkDataCount).Result()
//	result, serverCursor, err := s.RedisConnection.SScan(req.RowId, nextCursor, "", 300).Result()
//	if err != nil {
//		log.Println("getRows 5")
//		return nil, err
//	}
//	nextCursor = serverCursor
//	wg.Add(1)
//
//	go func(redisConn *redis.Client) error {
//
//		for _, k := range result {
//			//Important lesson, challenge was to convert interface{} to byte. used  ([]byte(k.(string)))
//			var movieTile pb.ContentTile
//			err = proto.UnmarshalText(k, &movieTile)
//			//err = proto.Unmarshal(([]byte(k.(string))), &movieTile)
//			if err != nil {
//				log.Println("getRows 6")
//				return err
//			}
//			rowResp.ContentTile = append(rowResp.ContentTile, &movieTile)
//		}
//		wg.Done()
//		return nil
//	}(s.RedisConnection)
//
//	wg.Wait()
//	if serverCursor == 0 {
//		break
//	}
//}
//return &rowResp, nil
//
//return  nil, nil