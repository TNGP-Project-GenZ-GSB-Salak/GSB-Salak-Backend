User {
	ID uuid pk
	Username string
	Fullname string
	CreatedAt timestamp
	UpdatedAt timestamp
	PasswordHash string
}

Account {
	ID uuid pk
	UserID uuid > User.ID
	AccountNumber string
	Type string
	Balance decimal
	Currency string
	CreatedAt timestamp
	UpdatedAt timestamp
}

Holding {
	AccountID uuid > Account.ID
	ProductID uuid > SalakProduct.ID
	Units int
	TicketLetter string
	TicketEnd int
	PurchaseAmount decimal
	PurchaseDate timestamp
	MaturityDate timestamp
	CreatedAt timestamp
	ID uuid pk
	TicketStart int
}

LedgerEntry {
	AccountID uuid > Account.ID
	HoldingID uuid > Holding.ID
	Type string
	Amount decimal
	BalanceAfter decimal
	ReferenceType string
	ReferenceID uuid
	Description string
	CreatedAt timestamp
	ID uuid pk
}

TicketSequence {
	ID int pk
	NextTicketNumber int
	UpdatedAt timestamp
}

SalakProduct {
	ID uuid pk
	Code string
	Name string
	TermMonths int
	UnitPrice decimal
	MinPurchase decimal
	MaxPurchase decimal
	StepAmount decimal
	IsActive bool
	CreatedAt timestamp
	UpdatedAt timestamp
}

// Type distinguishes 'deposit' vs 'withdrawal'. IsPremature is set only on a
// withdrawal taken before the Kapook goal/maturity is reached. The 2-per-
// calendar-year premature-withdrawal limit is enforced at withdrawal time by
// counting existing rows for the account (Type='withdrawal' AND
// IsPremature=true AND CreatedAt in the current calendar year) — no separate
// counter table, so there's nothing to keep in sync with this history.
TransactionSavingKapook {
	State string
	Type string
	IsPremature boolean
	Amount decimal
	UpdatedAt timestamp
	CreatedAt timestamp
	KapookAccountID uuid > Account.ID
	SourceAccountID uuid > Account.ID
	TranSavingID uuid pk
}

SavingKapook {
	KapookID uuid
	GoalAmount decimal
	SavingAmount decimal
	AccountID uuid > Account.ID
	Stat integer
	Status boolean
	SalakAmount decimal
}

AcceptTermCondition {
	ID uuid
	UserID uuid > User.ID
	AcceptDate timestamp
	CreatedAt timestamp
	UpdatedAt timestamp
}

Badge {
	BadgeID uuid pk
	ImageURL string
	CollectionID uuid > BadgeCategory.BadgeCategoryID
	CreatedAt timestamp
	UpdatedAt timestamp
}

HoldingBadge {
	CreatedAt timestamp
	UpdatedAt timestamp
	HoldingID uuid > Holding.ID
	BadgeID uuid > Badge.BadgeID
}

UserBadge {
	UserID uuid > User.ID
	BadgeID uuid > Badge.BadgeID
}

UserDefaultBadge {
	UserID uuid > User.ID
	BadgeID uuid > Badge.BadgeID
}

BadgeCategory {
	Name string
	CreatedAt timestamp
	UpdatedAt timestamp
	BadgeCategoryID uuid pk
}

