package gorm_generics

import (
	"context"
	"math"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormModel[E any] interface {
	ToEntity() E
	FromEntity(entity E) interface{}
}

type PageResult[M GormModel[E], E any] struct {
	Data  []M   `json:"data"`
	Count int64 `json:"count"`
	Page  int   `json:"page"`
}

type PageConfig struct {
	Page int   `json:"page"`
	Size int64 `json:"size"`
	// By default the count is generated when asking for the first page so that the user
	// receives the total amount, but with this we can optimize out this count
	IgnoreCount bool `json:"IngoreCount"`
	// if you want the count to always be returned.
	ForceCount bool `json:"ForceCount"`
}

func NewRepository[M GormModel[E], E any](db *gorm.DB) *GormRepository[M, E] {
	return &GormRepository[M, E]{
		db: db,
	}
}

type GormRepository[M GormModel[E], E any] struct {
	db *gorm.DB
}

func (r *GormRepository[M, E]) Insert(ctx context.Context, entity *E) error {
	var start M
	model := start.FromEntity(*entity).(M)

	err := r.db.WithContext(ctx).Create(&model).Error
	if err != nil {
		return err
	}

	*entity = model.ToEntity()
	return nil
}

func (r *GormRepository[M, E]) InsertDirect(ctx context.Context, entity *M) error {
	err := r.db.WithContext(ctx).Create(&entity).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *GormRepository[M, E]) InsertFromInterface(ctx context.Context, data interface{}) error {
	err := r.db.WithContext(ctx).Create(&data).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *GormRepository[M, E]) Delete(ctx context.Context, entity *E) error {
	var start M
	model := start.FromEntity(*entity).(M)
	err := r.db.WithContext(ctx).Delete(model).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *GormRepository[M, E]) DeleteById(ctx context.Context, id any) error {
	var start M
	err := r.db.WithContext(ctx).Delete(&start, &id).Error
	if err != nil {
		return err
	}

	return nil
}

func (r *GormRepository[M, E]) Update(ctx context.Context, entity *E) error {
	var start M
	model := start.FromEntity(*entity).(M)

	err := r.db.WithContext(ctx).Save(&model).Error
	if err != nil {
		return err
	}

	*entity = model.ToEntity()
	return nil
}

func (r *GormRepository[M, E]) UpdateDirect(ctx context.Context, entity *M) error {
	err := r.db.WithContext(ctx).Save(&entity).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *GormRepository[M, E]) FindByID(ctx context.Context, id any) (E, error) {
	var model M
	err := r.db.WithContext(ctx).First(&model, id).Error
	if err != nil {
		return *new(E), err
	}

	return model.ToEntity(), nil
}

func (r *GormRepository[M, E]) FindByIDWithOptions(ctx context.Context, id any, eagerLoad bool) (E, error) {
	var model M
	err := r.db.WithContext(ctx).Preload(clause.Associations).First(&model, id).Error
	if err != nil {
		return *new(E), err
	}

	return model.ToEntity(), nil
}

func (r *GormRepository[M, E]) FindByModel(ctx context.Context, entity *M) (M, error) {
	var model M
	err := r.db.WithContext(ctx).Preload(clause.Associations).Where(entity).First(&model).Error
	if err != nil {
		return *new(M), err
	}

	return model, err
}

func (r *GormRepository[M, E]) FindByModelMulti(ctx context.Context, entity *M) ([]M, error) {
	var models []M

	result := r.db.Where(&entity).Find(&models)
	if result.Error != nil {
		return nil, result.Error
	}
	return models, nil
}

func (r *GormRepository[M, E]) Find(ctx context.Context, specifications ...Specification) ([]E, error) {
	return r.FindWithLimit(ctx, -1, -1, specifications...)
}

func (r *GormRepository[M, E]) FindPaged(ctx context.Context, specifications ...Specification) ([]E, error) {
	return r.FindWithLimit(ctx, -1, -1, specifications...)
}

func (r *GormRepository[M, E]) Count(ctx context.Context, specifications ...Specification) (i int64, err error) {
	model := new(M)
	err = r.getPreWarmDbForSelect(ctx, specifications...).Model(model).Count(&i).Error
	return
}

func (r *GormRepository[M, E]) getPreWarmDbForSelect(ctx context.Context, specification ...Specification) *gorm.DB {
	var dbPrewarm *gorm.DB = r.db.WithContext(ctx)
	for _, s := range specification {
		dbPrewarm = dbPrewarm.Where(s.GetQuery(), s.GetValues()...)
	}
	return dbPrewarm
}

func (r *GormRepository[M, E]) FindWithLimit(ctx context.Context, limit int, offset int, specifications ...Specification) ([]E, error) {
	var models []M

	dbPrewarm := r.getPreWarmDbForSelect(ctx, specifications...)
	err := dbPrewarm.Limit(limit).Offset(offset).Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]E, 0, len(models))
	for _, row := range models {
		result = append(result, row.ToEntity())
	}

	return result, nil
}

func (r *GormRepository[M, E]) FindPagedWithLimit(ctx context.Context, pageCfg PageConfig, specifications ...Specification) (PageResult[M, E], error) {
	var models []M
	dbPrewarm := r.getPreWarmDbForSelect(ctx, specifications...)

	//If page is 0 do the count
	rs := PageResult[M, E]{
		Count: 0,
		Page:  pageCfg.Page,
	}

	minLimit := math.Max(1, float64(pageCfg.Size))
	shouldCount := pageCfg.ForceCount || (pageCfg.Page == 0 && !pageCfg.IgnoreCount)

	if shouldCount {
		model := new(M)

		var elementCount int64 = 0
		er := dbPrewarm.Model(model).Count(&elementCount)

		if er.Error != nil {
			return rs, er.Error
		}
		rs.Count = int64(math.Ceil(float64(elementCount) / float64(minLimit)))
	}

	err := dbPrewarm.Limit(int(minLimit)).Offset(pageCfg.Page).Find(&models).Error

	if err != nil {
		return rs, err
	}

	// result := make([]E, 0, len(models))
	// for _, row := range models {
	// 	result = append(result, row.ToEntity())
	// }

	rs.Data = models
	return rs, nil
}

func (r *GormRepository[M, E]) FindAll(ctx context.Context) ([]E, error) {
	return r.FindWithLimit(ctx, -1, -1)
}

func (r *GormRepository[M, E]) FindByEntity(ctx context.Context, e any) ([]E, error) {
	var models []M
	result := r.db.Where(&e).Find(&models)
	return r.FromModelToDto(models), result.Error
}

func (r *GormRepository[M, E]) FindByEntityWithOptions(ctx context.Context, e any, eagerLoad bool) ([]E, error) {
	var models []M
	result := r.db.Where(e).Preload(clause.Associations).Find(&models)
	return r.FromModelToDto(models), result.Error
}

func (r *GormRepository[M, E]) FromModelToDto(models []M) []E {
	result := make([]E, 0, len(models))

	if len(models) == 0 {
		return result
	}

	for _, row := range models {
		result = append(result, row.ToEntity())
	}
	return result
}
