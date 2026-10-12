// Реестр метрик (Capability) — то, что владелец выдаёт сотруднику точечно
// (EDR-0041 §3.1). Заменяет прежнюю Permission: право больше не приходит
// из роли как чёрный ящик, а перечислено здесь явно и показывается в
// матрице на странице сотрудника.
//
// Метрика — это одна «метрика/действие» из интерфейса. У большинства есть
// два уровня: LevelRead («видеть») и LevelWrite («менять»). У действий,
// которые не меняют данные, а совершают поступок (возврат, выгрузка,
// приглашение), уровней два: 'none' и 'action' — в матрице это один
// переключатель «можно». Это сделано намеренно: смешивать «видеть отчёт»
// и «оформить возврат» в одной строке значило бы выдать человеку лишнее.
//
// Проверка в транспорте — hasCapability(r, capability, level).
package auth

import "fmt"

// Level — уровень доступа к метрике.
type Level string

const (
	// LevelNone — метрика не выдана.
	LevelNone Level = ""
	// LevelRead — видеть данные метрики.
	LevelRead Level = "read"
	// LevelWrite — менять данные метрики. Всегда подразумевает LevelRead
	// (инвариант: «менять» без «видеть» невозможно, grant.go).
	LevelWrite Level = "write"
	// LevelAction — выполнить действие метрики (одноразовый поступок,
	// а не изменение данных). Не вложен в read/write.
	LevelAction Level = "action"
)

// Granted — уровень, который реально выдан (для action-метрик это
// LevelAction, для read/write — соответствующий уровень).
type Granted string

const (
	// GrantedNone — метрика не выдана. Значение пустое, а не «none»:
	// отсутствующий ключ в GrantSet даёт нулевое значение, и если бы
	// GrantedNone был «none», то HasAny() считал бы выданной любую
	// метрику, которой в наборе просто нет. В БД «none» тоже не
	// хранится: CHECK допускает только read/write/action.
	GrantedNone   Granted = ""
	GrantedRead   Granted = "read"
	GrantedWrite  Granted = "write"
	GrantedAction Granted = "action"
)

// Capability — именованная метрика админ-панели.
type Capability string

// Блоки интерфейса: по блокам собирается приветственный экран и правая
// колонка матрицы прав.
const (
	BlockSite       = "site"       // Сайт и витрина
	BlockPricing    = "pricing"    // Цены и расчёт
	BlockFinance    = "finance"    // Финансы
	BlockProduction = "production" // Производство
	BlockTraffic    = "traffic"    // Реклама и трафик
	BlockStaff      = "staff"      // Команда
	// Заголовок блока «projects» нужен из-за project.report: сами проекты
	// метрикой не являются (доступ персональный, см. выше).
	BlockProjects     = "projects"
	BlockIntegrations = "integrations"
)

// Метрики сайта и витрины.
const (
	CapSiteContacts  Capability = "site.contacts"  // контакты, адрес, часы работы
	CapSiteCompany   Capability = "site.company"   // реквизиты: ИНН, ОГРН, юрлицо
	CapSiteSocial    Capability = "site.social"    // ссылки на соцсети
	CapSiteSEO       Capability = "site.seo"       // SEO и счётчики аналитики
	CapSiteReviews   Capability = "site.reviews"   // отзывы клиентов
	CapSiteMaterials Capability = "site.materials" // каталог материалов и финишей
)

// Метрики цен и расчёта.
const (
	CapPriceMaterials Capability = "price.materials" // цены материалов ₽/кг
	CapPriceRates     Capability = "price.rates"     // ставки: себестоимость, наценка, НДС, скидка
	CapPriceServices  Capability = "price.services"  // услуги и тарифы
)

// Метрики финансов.
const (
	CapFinanceSales    Capability = "finance.sales"    // заказы и продажи
	CapFinancePayments Capability = "finance.payments" // платежи
	CapFinanceRefunds  Capability = "finance.refunds"  // возвраты (действие)
	CapFinanceCost     Capability = "finance.cost"     // себестоимость и прибыль
	CapDataExport      Capability = "data.export"      // выгрузка данных (действие)
)

// Метрики производства.
const (
	CapProductionReport Capability = "production.report" // раскрой, утилизация, отходы
	CapProductionPlan   Capability = "production.plan"   // задания в производство
)

// Метрики рекламы и трафика.
const (
	CapTrafficFunnel Capability = "traffic.funnel" // воронка витрины
	CapTrafficUsage  Capability = "traffic.usage"  // активность пользователей
)

// Метрики команды.
const (
	CapStaffView   Capability = "staff.view"   // видеть сотрудников
	CapStaffCreate Capability = "staff.create" // создать сотрудника (действие)
	CapStaffStatus Capability = "staff.status" // блокировать/разблокировать (действие)
	CapStaffRole   Capability = "staff.role"   // выдавать роли (действие)
	CapStaffGrants Capability = "staff.grants" // выдавать точечные права (действие)
	// Три метрики удаления разделены намеренно. staff.delete — мягкое
	// удаление: человек скрыт, вход закрыт, проекты переданы, история цела.
	// staff.delete_hard — безвозвратное сноса строки вместе с историей;
	// зарезервировано, потому что часть таблиц ссылается на сотрудника с
	// запретом удаления и включать такое без нужды нельзя.
	// staff.project_transfer — отдельное право передать проект целиком.
	CapStaffDelete          Capability = "staff.delete"           // удалить сотрудника (действие)
	CapStaffDeleteHard      Capability = "staff.delete_hard"      // удалить безвозвратно (действие)
	CapStaffProjectTransfer Capability = "staff.project_transfer" // передать проект (действие)
	CapSecurityPolicy       Capability = "security.policy"        // политика безопасности
	CapSecurityKeys         Capability = "security.keys"          // видеть API-ключи
	CapSecurityKeysIssue    Capability = "security.keys_issue"    // выдать или отозвать ключ (действие)
	CapAuditView            Capability = "audit.view"             // журнал аудита
)

// Метрики проектов.
const (
	// Проекты сознательно НЕ метрика: GET /projects отдаёт только проекты,
	// где вызывающий участник (project_members) — доступ и так персональный,
	// и выдавать «видеть проекты» было бы правом, которое нечего ограничивать.
	CapProjectReport Capability = "project.report" // сводка по всем проектам компании
)

// Метрики интеграций.
const (
	CapIntegrationEndpoints      Capability = "integration.endpoints"       // видеть ERP/MES-подключения
	CapIntegrationEndpointsIssue Capability = "integration.endpoints_issue" // добавить или удалить подключение (действие)
	CapStorageDelete             Capability = "storage.delete"              // удаление объектов хранилища (действие)
)

// CapabilitySpec — декларация метрики: как она называется в матрице,
// в каком блоке живёт и какие уровни допустимы.
type CapabilitySpec struct {
	Capability Capability
	Title      string
	Block      string
	// Levels — разрешённые уровни: либо {Read, Write}, либо {Action}.
	Levels []Level
}

// ReadWrite — метрика данных: видеть и менять.
func ReadWrite(c Capability, title, block string) CapabilitySpec {
	return CapabilitySpec{Capability: c, Title: title, Block: block, Levels: []Level{LevelRead, LevelWrite}}
}

// ReadOnly — метрика только для чтения (отчёты).
func ReadOnly(c Capability, title, block string) CapabilitySpec {
	return CapabilitySpec{Capability: c, Title: title, Block: block, Levels: []Level{LevelRead}}
}

// Action — метрика-действие: один переключатель «можно».
func Action(c Capability, title, block string) CapabilitySpec {
	return CapabilitySpec{Capability: c, Title: title, Block: block, Levels: []Level{LevelAction}}
}

// CapabilityRegistry — полный перечень метрик в порядке блоков: он же
// порядок строк в матрице прав, он же порядок плиток приветственного экрана.
var CapabilityRegistry = []CapabilitySpec{
	// Сайт и витрина
	ReadWrite(CapSiteContacts, "Контакты и адрес", BlockSite),
	ReadWrite(CapSiteCompany, "Реквизиты компании", BlockSite),
	ReadWrite(CapSiteSocial, "Соцсети", BlockSite),
	ReadWrite(CapSiteSEO, "SEO и счётчики", BlockSite),
	ReadWrite(CapSiteReviews, "Отзывы", BlockSite),
	ReadWrite(CapSiteMaterials, "Каталог материалов", BlockSite),
	// Цены и расчёт
	ReadWrite(CapPriceMaterials, "Цены материалов", BlockPricing),
	ReadWrite(CapPriceRates, "Ставки расчёта", BlockPricing),
	ReadWrite(CapPriceServices, "Услуги и тарифы", BlockPricing),
	// Финансы
	ReadWrite(CapFinanceSales, "Заказы и продажи", BlockFinance),
	ReadOnly(CapFinancePayments, "Платежи", BlockFinance),
	Action(CapFinanceRefunds, "Оформить возврат", BlockFinance),
	ReadOnly(CapFinanceCost, "Себестоимость и прибыль", BlockFinance),
	Action(CapDataExport, "Выгрузка данных", BlockFinance),
	// Производство
	ReadOnly(CapProductionReport, "Раскрой, утилизация, отходы", BlockProduction),
	ReadWrite(CapProductionPlan, "Задания в производство", BlockProduction),
	// Реклама и трафик
	ReadOnly(CapTrafficFunnel, "Воронка", BlockTraffic),
	ReadOnly(CapTrafficUsage, "Активность пользователей", BlockTraffic),
	// Команда
	ReadOnly(CapStaffView, "Сотрудники", BlockStaff),
	Action(CapStaffCreate, "Создать сотрудника", BlockStaff),
	Action(CapStaffStatus, "Блокировать сотрудника", BlockStaff),
	Action(CapStaffRole, "Выдавать роли", BlockStaff),
	Action(CapStaffGrants, "Выдавать точечные права", BlockStaff),
	Action(CapStaffDelete, "Удалить сотрудника", BlockStaff),
	Action(CapStaffDeleteHard, "Удалить безвозвратно", BlockStaff),
	Action(CapStaffProjectTransfer, "Передать проект", BlockStaff),
	ReadWrite(CapSecurityPolicy, "Политика безопасности", BlockStaff),
	ReadOnly(CapSecurityKeys, "API-ключи", BlockStaff),
	Action(CapSecurityKeysIssue, "Выдать или отозвать API-ключ", BlockStaff),
	ReadOnly(CapAuditView, "Журнал аудита", BlockStaff),
	// Проекты
	ReadOnly(CapProjectReport, "Сводка по проектам", BlockProjects),
	// Интеграции
	ReadOnly(CapIntegrationEndpoints, "ERP/MES-подключения", BlockIntegrations),
	Action(CapIntegrationEndpointsIssue, "Добавить или удалить подключение", BlockIntegrations),
	Action(CapStorageDelete, "Удаление файлов", BlockIntegrations),
}

// CapabilityTitles — заголовки блоков в порядке следования: используется
// фронтом для группировки строк матрицы и плиток.
var CapabilityBlocks = []struct {
	ID    string
	Title string
}{
	{BlockSite, "Сайт и витрина"},
	{BlockPricing, "Цены и расчёт"},
	{BlockFinance, "Финансы"},
	{BlockProduction, "Производство"},
	{BlockTraffic, "Реклама и трафик"},
	{BlockStaff, "Команда"},
	{BlockIntegrations, "Интеграции"},
	{BlockProjects, "Проекты"},
}

var capabilityIndex = func() map[Capability]CapabilitySpec {
	m := make(map[Capability]CapabilitySpec, len(CapabilityRegistry))
	for _, s := range CapabilityRegistry {
		m[s.Capability] = s
	}
	return m
}()

// LookupCapability возвращает декларацию метрики.
func LookupCapability(c Capability) (CapabilitySpec, bool) {
	s, ok := capabilityIndex[c]
	return s, ok
}

// AllowsLevel — допустим ли уровень для метрики по её декларации.
func AllowsLevel(c Capability, l Level) bool {
	s, ok := capabilityIndex[c]
	if !ok {
		return false
	}
	for _, allowed := range s.Levels {
		if allowed == l {
			return true
		}
	}
	return false
}

// IsAction — метрика-действие (один переключатель, без «видеть/менять»).
func IsAction(c Capability) bool {
	s, ok := capabilityIndex[c]
	if !ok {
		return false
	}
	return len(s.Levels) == 1 && s.Levels[0] == LevelAction
}

// ParseLevel нормализует уровень из строки DTO. Пустая строка и «none»
// означают «не выдано» — это позволяет фронту слать пустой набор.
func ParseLevel(s string) (Level, error) {
	switch Level(s) {
	// «Не выдано» приходит двумя способами: пустой строкой (метрику убрали
	// из набора) и словом none (клиент явно снял выдачу). Это одно и то же.
	case LevelNone, Level("none"):
		return LevelNone, nil
	case LevelRead:
		return LevelRead, nil
	case LevelWrite:
		return LevelWrite, nil
	case LevelAction:
		return LevelAction, nil
	default:
		return LevelNone, fmt.Errorf("auth: unknown level %q", s)
	}
}

// GrantedLevel — переводит выданный уровень в форму хранения в БД.
// «write» хранится как 'write' (он подразумевает read), действие — 'action'.
func GrantedLevel(l Level) Granted {
	switch l {
	case LevelNone:
		return GrantedNone
	case LevelRead:
		return GrantedRead
	case LevelWrite:
		return GrantedWrite
	case LevelAction:
		return GrantedAction
	default:
		return GrantedNone
	}
}

// Satisfies — проверка «выданного уровня хватает для запрошенного».
//
// Правила:
//   - ничего не выдано → ничего не удовлетворяет;
//   - 'read' удовлетворяет запрос на read;
//   - 'write' удовлетворяет read и write (инвариант «менять» подразумевает
//     «видеть»: grant.go не даст выдать write без read);
//   - 'action' — самостоятельный уровень: он не даёт ни read, ни write.
func Satisfies(granted Granted, want Level) bool {
	if want == LevelNone || granted == GrantedNone {
		return false
	}
	switch granted {
	case GrantedNone:
		return false
	case GrantedAction:
		return want == LevelAction
	case GrantedRead:
		return want == LevelRead
	case GrantedWrite:
		return want == LevelRead || want == LevelWrite
	default:
		return false
	}
}

// AllCapabilities — все метрики реестра (для шаблонов ролей и бэкапфилла).
func AllCapabilities() []Capability {
	out := make([]Capability, 0, len(CapabilityRegistry))
	for _, s := range CapabilityRegistry {
		out = append(out, s.Capability)
	}
	return out
}

// CapabilityStrings — имена всех метрик: ими валидируются скоупы API-ключей.
// Раньше ключами можно было выдать любой из 20 старых прав; теперь — любую
// из 33 метрик. Старые имена не принимаются: в базе ключей нет ни одного
// (total_api_keys = 0), поэтому переход стоит ноль данных.
func CapabilityStrings() []string {
	out := make([]string, 0, len(CapabilityRegistry))
	for _, s := range CapabilityRegistry {
		out = append(out, string(s.Capability))
	}
	return out
}

// IsKnownCapability — проверка имени метрики для DTO API-ключа.
func IsKnownCapability(s string) bool {
	_, ok := capabilityIndex[Capability(s)]
	return ok
}
