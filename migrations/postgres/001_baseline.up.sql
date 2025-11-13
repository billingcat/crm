--
-- PostgreSQL database dump
--

-- Dumped from database version 14.19 (Homebrew)
-- Dumped by pg_dump version 14.19 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: api_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.api_tokens (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    owner_id bigint NOT NULL,
    user_id bigint,
    token_prefix character varying(16) NOT NULL,
    token_hash character varying(64) NOT NULL,
    salt character varying(64) NOT NULL,
    name character varying(100),
    scope character varying(200),
    expires_at timestamp with time zone,
    last_used_at timestamp with time zone,
    disabled boolean DEFAULT false NOT NULL
);


--
-- Name: api_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.api_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: api_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.api_tokens_id_seq OWNED BY public.api_tokens.id;


--
-- Name: companies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.companies (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    background text,
    name text,
    customer_number text,
    invoice_email text,
    contact_invoice text,
    address1 text,
    address2 text,
    country text,
    vat_id text,
    invoice_opening text,
    invoice_currency text,
    invoice_tax_type text,
    invoice_footer text,
    zip text,
    city text,
    invoice_exemption_reason text,
    owner_id bigint,
    default_tax_rate text,
    supplier_number text
);


--
-- Name: companies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.companies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: companies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.companies_id_seq OWNED BY public.companies.id;


--
-- Name: contact_infos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.contact_infos (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    owner_id bigint,
    value character varying(300),
    label character varying(100),
    parent_id bigint,
    parent_type character varying(50),
    type character varying(30),
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);


--
-- Name: invitations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invitations (
    id bigint NOT NULL,
    token text NOT NULL,
    email text,
    expires_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: invitations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.invitations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: invitations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.invitations_id_seq OWNED BY public.invitations.id;


--
-- Name: invoicepositions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invoicepositions (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    invoice_id bigint,
    "position" bigint,
    unit_code text,
    text text,
    quantity text,
    tax_rate text,
    net_price text,
    gross_price text,
    line_total text,
    owner_id bigint
);


--
-- Name: invoicepositions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.invoicepositions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: invoicepositions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.invoicepositions_id_seq OWNED BY public.invoicepositions.id;


--
-- Name: invoices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invoices (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    number text,
    date timestamp with time zone,
    occurrence_date timestamp with time zone,
    suppliernumber text,
    contact_invoice text,
    ordernumber text,
    opening text,
    footer text,
    tax_type text,
    currency text,
    tax_number text,
    company_id bigint,
    net_total text,
    gross_total text,
    due_date timestamp with time zone,
    exemption_reason text,
    owner_id bigint,
    counter bigint,
    order_number text,
    supplier_number text,
    status text DEFAULT 'draft'::text NOT NULL,
    issued_at timestamp with time zone,
    paid_at timestamp with time zone,
    voided_at timestamp with time zone,
    template_id bigint,
    CONSTRAINT chk_invoices_status CHECK ((status = ANY (ARRAY['draft'::text, 'issued'::text, 'paid'::text, 'voided'::text])))
);


--
-- Name: invoices_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.invoices_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: invoices_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.invoices_id_seq OWNED BY public.invoices.id;


--
-- Name: letterhead_regions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.letterhead_regions (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    template_id bigint,
    owner_id bigint,
    kind text,
    page bigint,
    x_cm numeric,
    y_cm numeric,
    width_cm numeric,
    height_cm numeric,
    h_align character varying(10),
    v_align character varying(10),
    font_name character varying(50),
    font_size_pt numeric,
    line_spacing numeric,
    has_page2 boolean DEFAULT false NOT NULL,
    x2_cm numeric,
    y2_cm numeric,
    width2_cm numeric,
    height2_cm numeric
);


--
-- Name: letterhead_regions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.letterhead_regions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: letterhead_regions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.letterhead_regions_id_seq OWNED BY public.letterhead_regions.id;


--
-- Name: letterhead_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.letterhead_templates (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    owner_id bigint,
    name character varying(200),
    page_width_cm numeric,
    page_height_cm numeric,
    pdf_path text,
    preview_page1_url text,
    preview_page2_url text,
    font_normal character varying(255),
    font_bold character varying(255),
    font_italic character varying(255)
);


--
-- Name: letterhead_templates_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.letterhead_templates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: letterhead_templates_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.letterhead_templates_id_seq OWNED BY public.letterhead_templates.id;


--
-- Name: notes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notes (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    owner_id bigint,
    author_id bigint,
    parent_id bigint,
    parent_type text,
    title text,
    body text,
    tags text,
    edited_at timestamp with time zone
);


--
-- Name: notes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.notes ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.notes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: people; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.people (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text,
    "position" text,
    e_mail text,
    company_id bigint,
    owner_id bigint
);


--
-- Name: people_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.people_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: people_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.people_id_seq OWNED BY public.people.id;


--
-- Name: phones_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.phones_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: phones_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.phones_id_seq OWNED BY public.contact_infos.id;


--
-- Name: recent_views; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.recent_views (
    user_id bigint NOT NULL,
    entity_type text NOT NULL,
    entity_id bigint NOT NULL,
    viewed_at timestamp with time zone NOT NULL
);




--
-- Name: settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settings (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    company_name text,
    invoice_contact text,
    address1 text,
    address2 text,
    city text,
    country_code text,
    vat_id text,
    tax_number text,
    invoice_number_template text,
    use_local_counter boolean,
    bank_iban text,
    bank_name text,
    bank_bic text,
    owner_id bigint,
    invoice_email text,
    zip text,
    customer_number_prefix text,
    customer_number_width integer,
    customer_number_counter bigint
);


--
-- Name: settings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: settings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.settings_id_seq OWNED BY public.settings.id;


--
-- Name: signup_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.signup_tokens (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    email text NOT NULL,
    token_hash bytea NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone,
    password_hash text NOT NULL
);


--
-- Name: signup_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.signup_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: signup_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.signup_tokens_id_seq OWNED BY public.signup_tokens.id;


--
-- Name: tag_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_links (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    owner_id bigint NOT NULL,
    tag_id bigint NOT NULL,
    parent_type character varying(32) NOT NULL,
    parent_id bigint NOT NULL
);


--
-- Name: tag_links_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_links_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_links_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_links_id_seq OWNED BY public.tag_links.id;


--
-- Name: tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tags (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    owner_id bigint,
    name character varying(128) NOT NULL,
    norm character varying(128) NOT NULL
);


--
-- Name: tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tags_id_seq OWNED BY public.tags.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    email text NOT NULL,
    password_reset_token bytea,
    password_reset_expiry timestamp with time zone,
    password text NOT NULL,
    full_name text,
    verified boolean DEFAULT false NOT NULL,
    last_login_at timestamp with time zone,
    owner_id bigint,
    approved_at timestamp with time zone
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: api_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_tokens ALTER COLUMN id SET DEFAULT nextval('public.api_tokens_id_seq'::regclass);


--
-- Name: companies id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.companies ALTER COLUMN id SET DEFAULT nextval('public.companies_id_seq'::regclass);


--
-- Name: contact_infos id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.contact_infos ALTER COLUMN id SET DEFAULT nextval('public.phones_id_seq'::regclass);


--
-- Name: invitations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitations ALTER COLUMN id SET DEFAULT nextval('public.invitations_id_seq'::regclass);


--
-- Name: invoicepositions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoicepositions ALTER COLUMN id SET DEFAULT nextval('public.invoicepositions_id_seq'::regclass);


--
-- Name: invoices id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices ALTER COLUMN id SET DEFAULT nextval('public.invoices_id_seq'::regclass);


--
-- Name: letterhead_regions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.letterhead_regions ALTER COLUMN id SET DEFAULT nextval('public.letterhead_regions_id_seq'::regclass);


--
-- Name: letterhead_templates id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.letterhead_templates ALTER COLUMN id SET DEFAULT nextval('public.letterhead_templates_id_seq'::regclass);


--
-- Name: people id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.people ALTER COLUMN id SET DEFAULT nextval('public.people_id_seq'::regclass);


--
-- Name: settings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settings ALTER COLUMN id SET DEFAULT nextval('public.settings_id_seq'::regclass);


--
-- Name: signup_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.signup_tokens ALTER COLUMN id SET DEFAULT nextval('public.signup_tokens_id_seq'::regclass);


--
-- Name: tag_links id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_links ALTER COLUMN id SET DEFAULT nextval('public.tag_links_id_seq'::regclass);


--
-- Name: tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags ALTER COLUMN id SET DEFAULT nextval('public.tags_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: api_tokens api_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_tokens
    ADD CONSTRAINT api_tokens_pkey PRIMARY KEY (id);


--
-- Name: signup_tokens idx_16491_signup_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.signup_tokens
    ADD CONSTRAINT idx_16491_signup_tokens_pkey PRIMARY KEY (id);


--
-- Name: people idx_16697_people_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.people
    ADD CONSTRAINT idx_16697_people_pkey PRIMARY KEY (id);


--
-- Name: settings idx_16704_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settings
    ADD CONSTRAINT idx_16704_settings_pkey PRIMARY KEY (id);


--
-- Name: invoicepositions idx_16711_invoicepositions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoicepositions
    ADD CONSTRAINT idx_16711_invoicepositions_pkey PRIMARY KEY (id);


--
-- Name: invoices idx_16718_invoices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT idx_16718_invoices_pkey PRIMARY KEY (id);


--
-- Name: companies idx_16725_companies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.companies
    ADD CONSTRAINT idx_16725_companies_pkey PRIMARY KEY (id);


--
-- Name: contact_infos idx_16732_phones_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.contact_infos
    ADD CONSTRAINT idx_16732_phones_pkey PRIMARY KEY (id);


--
-- Name: users idx_16739_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT idx_16739_users_pkey PRIMARY KEY (id);


--
-- Name: notes idx_16750_notes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT idx_16750_notes_pkey PRIMARY KEY (id);


--
-- Name: invitations invitations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitations
    ADD CONSTRAINT invitations_pkey PRIMARY KEY (id);


--
-- Name: invitations invitations_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitations
    ADD CONSTRAINT invitations_token_key UNIQUE (token);


--
-- Name: letterhead_regions letterhead_regions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.letterhead_regions
    ADD CONSTRAINT letterhead_regions_pkey PRIMARY KEY (id);


--
-- Name: letterhead_templates letterhead_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.letterhead_templates
    ADD CONSTRAINT letterhead_templates_pkey PRIMARY KEY (id);

--
-- Name: tag_links tag_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_links
    ADD CONSTRAINT tag_links_pkey PRIMARY KEY (id);


--
-- Name: tags tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_pkey PRIMARY KEY (id);


--
-- Name: idx_16491_idx_signup_tokens_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16491_idx_signup_tokens_deleted_at ON public.signup_tokens USING btree (deleted_at);


--
-- Name: idx_16491_idx_signup_tokens_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16491_idx_signup_tokens_email ON public.signup_tokens USING btree (email);


--
-- Name: idx_16491_idx_signup_tokens_token_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_16491_idx_signup_tokens_token_hash ON public.signup_tokens USING btree (token_hash);


--
-- Name: idx_16697_idx_people_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16697_idx_people_deleted_at ON public.people USING btree (deleted_at);


--
-- Name: idx_16704_idx_settings_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16704_idx_settings_deleted_at ON public.settings USING btree (deleted_at);


--
-- Name: idx_16718_idx_invoices_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16718_idx_invoices_deleted_at ON public.invoices USING btree (deleted_at);


--
-- Name: idx_16725_idx_companies_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16725_idx_companies_deleted_at ON public.companies USING btree (deleted_at);


--
-- Name: idx_16739_idx_users_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16739_idx_users_deleted_at ON public.users USING btree (deleted_at);


--
-- Name: idx_16739_idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_16739_idx_users_email ON public.users USING btree (email);


--
-- Name: idx_16745_idx_recent_user_viewed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16745_idx_recent_user_viewed_at ON public.recent_views USING btree (user_id, viewed_at);


--
-- Name: idx_16745_idx_user_view; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16745_idx_user_view ON public.recent_views USING btree (user_id, entity_type, entity_id);


--
-- Name: idx_16745_idx_user_viewed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16745_idx_user_viewed_at ON public.recent_views USING btree (viewed_at);


--
-- Name: idx_16745_ux_recent_user_entity; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_16745_ux_recent_user_entity ON public.recent_views USING btree (user_id, entity_type, entity_id);


--
-- Name: idx_16750_idx_notes_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16750_idx_notes_author_id ON public.notes USING btree (author_id);


--
-- Name: idx_16750_idx_notes_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_16750_idx_notes_deleted_at ON public.notes USING btree (deleted_at);


--
-- Name: idx_api_tokens_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_tokens_deleted_at ON public.api_tokens USING btree (deleted_at);


--
-- Name: idx_api_tokens_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_tokens_owner_id ON public.api_tokens USING btree (owner_id);


--
-- Name: idx_api_tokens_token_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_api_tokens_token_hash ON public.api_tokens USING btree (token_hash);


--
-- Name: idx_api_tokens_token_prefix; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_tokens_token_prefix ON public.api_tokens USING btree (token_prefix);


--
-- Name: idx_api_tokens_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_tokens_user_id ON public.api_tokens USING btree (user_id);


--
-- Name: idx_companies_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_companies_deleted_at ON public.companies USING btree (deleted_at);


--
-- Name: idx_contact_infos_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contact_infos_deleted_at ON public.contact_infos USING btree (deleted_at);


--
-- Name: idx_contact_infos_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contact_infos_owner_id ON public.contact_infos USING btree (owner_id);


--
-- Name: idx_contact_infos_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contact_infos_type ON public.contact_infos USING btree (type);


--
-- Name: idx_contact_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contact_parent ON public.contact_infos USING btree (parent_type, parent_id);


--
-- Name: idx_invoices_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_invoices_deleted_at ON public.invoices USING btree (deleted_at);


--
-- Name: idx_invoices_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_invoices_status ON public.invoices USING btree (status);


--
-- Name: idx_letterhead_templates_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_letterhead_templates_deleted_at ON public.letterhead_templates USING btree (deleted_at);


--
-- Name: idx_letterhead_templates_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_letterhead_templates_owner_id ON public.letterhead_templates USING btree (owner_id);


--
-- Name: idx_notes_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notes_author_id ON public.notes USING btree (author_id);


--
-- Name: idx_notes_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notes_deleted_at ON public.notes USING btree (deleted_at);


--
-- Name: idx_owner_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_owner_status ON public.invoices USING btree (status);


--
-- Name: idx_people_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_people_deleted_at ON public.people USING btree (deleted_at);


--
-- Name: idx_recent_user_viewed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_recent_user_viewed_at ON public.recent_views USING btree (user_id, viewed_at DESC);


--
-- Name: idx_regions_tpl_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_regions_tpl_owner ON public.letterhead_regions USING btree (template_id, owner_id);


--
-- Name: idx_settings_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_settings_deleted_at ON public.settings USING btree (deleted_at);


--
-- Name: idx_settings_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_settings_owner_id ON public.settings USING btree (owner_id);


--
-- Name: idx_signup_tokens_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_signup_tokens_deleted_at ON public.signup_tokens USING btree (deleted_at);


--
-- Name: idx_signup_tokens_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_signup_tokens_email ON public.signup_tokens USING btree (email);


--
-- Name: idx_signup_tokens_token_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_signup_tokens_token_hash ON public.signup_tokens USING btree (token_hash);


--
-- Name: idx_tag_links_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tag_links_deleted_at ON public.tag_links USING btree (deleted_at);


--
-- Name: idx_tag_owner_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tag_owner_name ON public.tags USING btree (owner_id, name);


--
-- Name: idx_taglink_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_taglink_parent ON public.tag_links USING btree (parent_type, parent_id);


--
-- Name: idx_taglink_tag; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_taglink_tag ON public.tag_links USING btree (tag_id);


--
-- Name: idx_user_view; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_view ON public.recent_views USING btree (user_id, entity_type, entity_id);


--
-- Name: idx_user_viewed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_viewed_at ON public.recent_views USING btree (viewed_at);


--
-- Name: idx_users_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_deleted_at ON public.users USING btree (deleted_at);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_users_email ON public.users USING btree (email);


--
-- Name: uniq_tag_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uniq_tag_parent ON public.tag_links USING btree (owner_id, tag_id, parent_type, parent_id);


--
-- Name: uniq_tag_per_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uniq_tag_per_owner ON public.tags USING btree (owner_id, norm);


--
-- Name: uniq_tpl_owner_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uniq_tpl_owner_kind ON public.letterhead_regions USING btree (template_id, owner_id, kind);


--
-- Name: ux_recent_user_entity; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_recent_user_entity ON public.recent_views USING btree (user_id, entity_type, entity_id);


--
-- Name: invoices fk_companies_invoices; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT fk_companies_invoices FOREIGN KEY (company_id) REFERENCES public.companies(id);


--
-- Name: invoicepositions fk_invoices_invoice_positions; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoicepositions
    ADD CONSTRAINT fk_invoices_invoice_positions FOREIGN KEY (invoice_id) REFERENCES public.invoices(id);


--
-- Name: invoices fk_invoices_template; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT fk_invoices_template FOREIGN KEY (template_id) REFERENCES public.letterhead_templates(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: letterhead_regions fk_letterhead_templates_regions; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.letterhead_regions
    ADD CONSTRAINT fk_letterhead_templates_regions FOREIGN KEY (template_id) REFERENCES public.letterhead_templates(id) ON DELETE CASCADE;


--
-- Name: people fk_people_company; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.people
    ADD CONSTRAINT fk_people_company FOREIGN KEY (company_id) REFERENCES public.companies(id);


--
-- Name: tag_links fk_tag_links_tag; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_links
    ADD CONSTRAINT fk_tag_links_tag FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: invoicepositions invoicepositions_invoice_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoicepositions
    ADD CONSTRAINT invoicepositions_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES public.invoices(id);


--
-- Name: invoices invoices_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id);


--
-- Name: people people_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.people
    ADD CONSTRAINT people_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id);


--
-- PostgreSQL database dump complete
--

