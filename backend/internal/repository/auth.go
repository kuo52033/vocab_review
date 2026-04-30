package repository

import "vocabreview/backend/internal/domain"

func (s *Store) UpsertUser(user domain.User) error {
	s.mu.Lock()
	s.Users[user.ID] = user
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) FindUserByEmail(email string) (domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.Users {
		if user.Email == email {
			return user, true
		}
	}
	return domain.User{}, false
}

func (s *Store) PutMagicLink(token domain.MagicLinkToken) error {
	s.mu.Lock()
	s.MagicLinks[token.Token] = token
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) GetMagicLink(token string) (domain.MagicLinkToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.MagicLinks[token]
	return value, ok
}

func (s *Store) DeleteMagicLink(token string) error {
	s.mu.Lock()
	delete(s.MagicLinks, token)
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) PutSession(session domain.Session) error {
	s.mu.Lock()
	s.Sessions[session.Token] = session
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) GetSession(token string) (domain.Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.Sessions[token]
	return session, ok
}
