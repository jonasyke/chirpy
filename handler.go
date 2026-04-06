package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jonasyke/chirpy/internal/auth"
	"github.com/jonasyke/chirpy/internal/database"
)

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	hitCount := cfg.fileserverHits.Load()

	html := fmt.Sprintf(`
		<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
		</html>
		`, hitCount)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))

}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, 403, "You do not have the credentials")
		return
	}
	err := cfg.db.ResetUser(r.Context())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not reset users")
		return
	}

	cfg.fileserverHits.Store(0)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hit count reset"))
}

func (cfg *apiConfig) handlerValidate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Message string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong in Validation")
		return
	}

	cleaned, err := validateChirp(params.Message)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	type returnVals struct {
		CleanedBody string `json:"cleaned_body"`
	}

	respondWithJSON(w, 200, returnVals{CleanedBody: cleaned})
}

func (cfg *apiConfig) handlerUsersCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong in User Creation")
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not process password")
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
			Email: params.Email,
			HashedPassword: hashedPassword,
		})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not Create User")
		return
	}

	respondWithJSON(w, 201, User{
		ID:         user.ID,
		Created_at: user.CreatedAt,
		Updated_at: user.UpdatedAt,
		Email:      user.Email,
	})

}

func (cfg *apiConfig) handlerChirpsCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body    string    `json:"body"`
	}

		token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "unauthorized")
		return
	}

	validID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, "unauthorized")
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "problem when decoding chirp")
		return
	}

	validated, err := validateChirp(params.Body)
	if err != nil {
		respondWithError(w, 406, err.Error())
		return
	}

	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   validated,
		UserID: validID,
	})
	if err != nil {
		respondWithError(w, 406, "Could not create chirp")
		return
	}

	respondWithJSON(w, 201, chirpResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})

}

func (cfg *apiConfig) handlerChirpsRetrieve(w http.ResponseWriter, r *http.Request) {

	processedChirps := []chirpResponse{}

	bulkChrips, err := cfg.db.GetChirps(r.Context())
	if err != nil {
		respondWithError(w, 500, "Could not retrieve chirps")
		return
	}

	for _, chirp := range bulkChrips {
		processedChirps = append(processedChirps, chirpResponse{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	}
	respondWithJSON(w, 200, processedChirps)
}

func (cfg *apiConfig) handlerChirpsGet(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	parsedID, err := uuid.Parse(chirpID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not find chirp")
		return
	}
	
	chirp, err := cfg.db.GetChirp(r.Context(), parsedID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "could not find retrieve chirp")
		return
	}
	respondWithJSON(w, http.StatusOK, chirpResponse{
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
	})
}

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type loginResponse struct {
		ID uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email string `json:"email"`
		Token string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	
	type parameters struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not process request")
		return
	}

	user, err := cfg.db.LoginLookup(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	
	cleared, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	
	if !cleared {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to verify access")
		return
	}

	initalToken := auth.MakeRefreshToken()

	finalToken, err := cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token: initalToken,
		UserID: user.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour * 24 * 60),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	respondWithJSON(w, 200, loginResponse{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
		Token: token,
		RefreshToken: finalToken.Token,
	})

}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusForbidden, "Could not validate Token")
		return
	}
	user, err := cfg.db.GetUserFromRefreshToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, 401, "could not validate Token")
		return
	}

	newToken, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to generate new token")
		return
	}
	type response struct {
		Token string `json:"token"`
	}
	respondWithJSON(w, 200, response{
		Token: newToken,
	})
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusForbidden, "Could not Validate Token")
		return
	}
	_, err = cfg.db.RevokeRefreshToken(r.Context(), refreshToken)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerUpdateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong in User Update")
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not process password")
		return
	}
	
	givenToken, err:= auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Could not Retrieve Credentials")
		return
	}

	validatedUser, err := auth.ValidateJWT(givenToken, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Could not Validate Credentials")
		return
	}

	user, err := cfg.db.UpdateUser(r.Context(), database.UpdateUserParams{
		ID: validatedUser,
		HashedPassword: hashedPassword,
		Email: params.Email,
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not update User")
		return
	}

	respondWithJSON(w, http.StatusOK, User{
		ID: user.ID,
		Created_at: user.CreatedAt,
		Updated_at: user.UpdatedAt,
		Email: user.Email,
	})
}

func (cfg *apiConfig) handlerDeleteChirp(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	chirpIDString := r.PathValue("chirpID")
	
	chirpID, err := uuid.Parse(chirpIDString)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Could not find chirp")
		return
	}

	dbChirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "could not find chirp")
		return
	}

	if dbChirp.UserID != userID {
		respondWithError(w, http.StatusForbidden, "unauthorized")
		return
	}

	err = cfg.db.DeleteChirp(r.Context(), dbChirp.ID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not delete chirp")
		return
	}
	
	w.WriteHeader(http.StatusNoContent)

}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{Error: msg})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	if payload == nil {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Encode error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(data)
}
